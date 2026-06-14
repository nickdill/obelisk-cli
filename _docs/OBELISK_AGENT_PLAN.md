# Obelisk Agent — Spec

The `obelisk-agent` is a lightweight Go HTTP service that runs as a Docker container
inside every deployed Obelisk. It is the only way the `obelisk` CLI mutates a live server.

---

## Deployment

The agent is **not built in this repo**. It lives at `nickdill/obelisk-agent` and ships
as a Docker image: `ghcr.io/nickdill/obelisk-agent:<version>`.

It is added as a service in the Obelisk template's `docker-compose.yml`:

```yaml
services:
  obelisk-agent:
    image: ghcr.io/nickdill/obelisk-agent:latest
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock  # to restart containers
      - .:/obelisk                                  # project dir for scripts + config
      - ./data/agent:/data                          # persistent: allowed_keys.json, audit.log
    networks:
      - obelisk
    expose:
      - "9100"
```

nginx proxies the reserved path to the agent (internal docker network only — the agent is
never exposed to the internet directly):

```nginx
location /_obelisk/ {
  proxy_pass http://obelisk-agent:9100/;
}
```

---

## Auth Protocol

Every request must carry four headers. The agent rejects any request that fails validation
before executing any operation.

| Header | Content |
|---|---|
| `X-Obelisk-Key` | Sender's public key (`obk1_...`) |
| `X-Obelisk-Timestamp` | Unix seconds at time of signing |
| `X-Obelisk-Nonce` | 16 random bytes, hex-encoded |
| `X-Obelisk-Signature` | base64 ED25519 signature over the canonical string |

**Canonical signing string** (newline-separated):

```
v1
{METHOD}
{PATH}
{timestamp}
{nonce}
{hex(sha256(request body))}
```

**`PATH` is the agent-side escaped path** (e.g. `/v1/deploy`) — never the `/_obelisk` proxy prefix. This keeps signatures proxy-agnostic. Use `r.URL.EscapedPath()` on both sides.

**Verification order (fail fast):**

1. Timestamp window: `|now − timestamp| > 60s` → `401 stale_timestamp`
2. Key lookup: key not in `allowed_keys.json` → `403 unknown_key`
3. Signature: ED25519 verify over recomputed canonical string → `401 bad_signature`. Body capped at 1 MiB → `413 body_too_large`.
4. Replay check (committed post-verify): `(key, nonce)` seen within last 120s → `401 replay`. Cache capped at 100k entries → `503 server_busy`.

Nonce is committed only after the signature verifies — failed requests don't burn their nonce.

All errors return JSON: `{"error": "<code>", "message": "<human readable>"}`.

Every response includes `X-Obelisk-Agent-Version` so the CLI can warn on protocol drift.

See `CLI_AUTHENTICATION_PLAN.md` §2–3 for the full identity model and wire protocol spec. Golden vectors at `obelisk-agent/internal/auth/testdata/vectors.json` are the executable form of the spec.

---

## v1 API

All routes are under `/_obelisk/` externally → `/v1/` on the agent.

| Endpoint | Purpose |
|---|---|
| `GET /v1/ping` | Auth check + handshake. Returns: `{"agent_version": "...", "server_name": "...", "protocol": "v1"}` |
| `GET /v1/status` | Module list with container states (`running` / `exited` / health). Sourced from `docker compose ps`. |
| `POST /v1/deploy` | Invoke `.obelisk/run.sh` (or `setup.sh` for a new module) to pull the latest image/source, regenerate configs, restart containers, and reload nginx. Body: `{"module": "api"}` or `{}` for all modules. |
| `GET /v1/keys` | List authorized keys: name, fingerprint, added_at, added_by. |
| `POST /v1/keys` | Authorize a key. Body: `{"key": "obk1_...", "name": "Teammate Bob"}`. |
| `DELETE /v1/keys/{fingerprint}` | Revoke a key. Refuses to delete the last remaining key (lockout guard). |

**MVP required for ship:** `ping` and `deploy`. Status + key management are Phase 2.

### Deploy behavior

`POST /v1/deploy` invokes the existing `.obelisk/` scripts from the mounted project directory.
This keeps one source of truth — no docker commands duplicated in the agent.

- New module (not yet in compose): runs `.obelisk/setup.sh` then `.obelisk/run.sh`
- Existing module update: runs `.obelisk/run.sh` directly
- Body `{}` (all modules): runs `run.sh` for the full stack

The agent streams script stdout/stderr into the response body as it runs. The **last line** of the stream is always `{"exit_code":N}` on its own line. Deploys are serialized — a second concurrent request returns `409 deploy_in_progress`. Deploys time out after 30 minutes.

---

## Persistence

All persistent state lives in `/data/` inside the container, mounted from `./data/agent/`
on the host. Survives redeploys, image updates, and container restarts.

**`/data/allowed_keys.json`**

```json
{
  "version": 1,
  "keys": [
    {
      "key": "obk1_MCowBQYDK2VwAyEA...",
      "name": "Nick (laptop)",
      "added_at": "2026-06-09T20:00:00Z",
      "added_by": "SHA256:bootstrap"
    }
  ]
}
```

- `version` is reserved for future role/scope additions without breaking v1 agents.
- All writes are atomic: write to a temp file in the same directory, then `os.Rename`.

**`/data/audit.log`**

Every authenticated mutating operation is appended (one JSON object per line):

```json
{"ts": "2026-06-09T20:01:00Z", "fingerprint": "SHA256:abc...", "op": "deploy", "module": "api", "result": "ok"}
```

---

## Bootstrap (First Key)

The trust chain starts at provisioning, when the developer already controls the server via SSH.

1. Dev runs `obelisk identity` locally → copies the `obk1_...` public key string
2. Pastes it into the Obelisk template's `.env`:
   ```
   OBELISK_AUTHORIZED_KEY="obk1_... Nick (laptop)"
   ```
3. `setup.sh` seeds `data/agent/allowed_keys.json` from this value **only if the file does
   not already exist** — idempotent, never clobbers a live key list on re-runs.
4. From this point forward, all key management goes through the signed API (`obelisk allow` /
   `obelisk revoke`). SSH is not required again for Obelisk operations.

---

## Agent Repo Structure (`nickdill/obelisk-agent`)

```
obelisk-agent/
  main.go                   # HTTP server on :9100, wires routes + auth middleware
  go.mod                    # module: github.com/nickdill/obelisk-agent
  Dockerfile                # multi-stage: go build → scratch image with CA certs only
  .github/
    workflows/
      release.yml           # builds and pushes image to ghcr.io on version tag

  internal/
    auth/
      middleware.go         # Verifier: 4-check chain (timestamp, key, signature, nonce-commit)
      keys.go               # KeyStore: atomic writes, NormalizeKey, reload-on-miss, sentinel errors
      nonce.go              # NonceCache: 120s TTL, 100k cap, bounded incremental prune
      identity.go           # Identity type passed to handlers via request context
      testdata/
        vectors.json        # golden vectors — cross-repo wire protocol contract
        gen_vectors.go      # regenerates vectors.json (run only on deliberate protocol changes)
    api/
      ping.go               # GET /v1/ping
      deploy.go             # POST /v1/deploy — serialized, 30-min timeout, process-group kill
      status.go             # GET /v1/status — docker compose ps parsing (NDJSON + array)
      keys.go               # GET/POST/DELETE /v1/keys
      respond.go            # writeJSON/writeError helpers
```

**Status: ✅ built and committed on `develop` branch (`17ab3fd`).**

**Dependencies — stdlib only:**

- `crypto/ed25519` — signature verification
- `encoding/json`, `net/http`, `os/exec` — all standard library
- No HTTP framework. No third-party router.

**Image target:** scratch + CA certificates bundle only. Binary is statically linked.
Goal: under 10 MB image.

---

## Verification

**Local smoke test:**

```bash
docker build -t obelisk-agent:dev .
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/testdata:/obelisk \
  -v $(pwd)/testdata/agent:/data \
  -p 9100:9100 \
  obelisk-agent:dev
```

**Unit tests (agent repo):**

- Sign/verify round trip with golden test vectors shared with the CLI repo
- Every rejection path: stale timestamp, replayed nonce, unknown key, tampered body, bad signature
- `allowed_keys.json` atomic write and last-key lockout guard

**Integration (docker-compose in agent repo):**

- Compose file: agent + minimal fake project dir with stub `.obelisk/` scripts
- CLI integration tests hit `http://localhost:9100` (HTTP allowed for localhost per the auth plan)
- Sequence: `ping` → `deploy` → verify script was invoked → `keys` CRUD

**End-to-end:**

Provision a test Obelisk with the updated template. Seed one key via `.env` + `setup.sh`.
Then from a developer laptop, run:

```
obelisk server add test https://test.obelisk.example.com   # ping handshake
obelisk list                                                # status fan-out
obelisk deploy --server test                               # signed deploy call
```

Confirm the module update goes live with no SSH.

---

## Out of Scope (v1)

- **Log streaming** (`GET /v1/logs`) — Phase 2
- **Module add/remove via API** — modules are declared in `obelisk.yml` in the project dir; the agent reads what's there, it doesn't manage the list
- **Key roles / per-module permissions** — reserved for `allowed_keys.json` v2
- **Server provisioning** — spinning up new EC2/Droplet instances
- **Megalisk integration** — central control plane, billing, accounts
- **Passphrase-encrypted private keys** — noted as future hardening in the auth plan
