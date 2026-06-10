# CLI ↔ Obelisk Authentication Plan

Design for securing communication between the local `obelisk` CLI and deployed Obelisk servers, so developers can run `obelisk deploy`, `obelisk list`, and friends against production servers — including teammates — with no accounts, no central login service, and no passwords.

**Decisions locked in:**

| Decision | Choice |
|---|---|
| Auth model | Decentralized ED25519 keypairs (SSH authorized-keys style). No Megalisk accounts. |
| Scope | Full system: CLI signing **and** the server-side verifying agent. |
| Agent location | Separate `obelisk-agent` repo (own Go module, Dockerfile, releases). This doc is the wire-protocol contract between the repos. |
| Bootstrap trust | First public key baked in at provisioning via the Obelisk template's `.env` + `setup.sh`. |
| Server registry | Local-only, in `~/.config/obelisk/`. Nothing committed to git. |
| Permissions | All keys equal in v1. No roles. (Format is versioned so roles can be added later.) |

---

## 1. Overview

### The problem

A deployed Obelisk is a docker-compose stack (nginx + certbot + module containers) on a server you own. Today, updating it means SSH or CodeDeploy. The goal is:

```
cd my-project && obelisk deploy     # pushes the update to the deployed Obelisk
obelisk list                        # shows all your Obelisks and their modules
```

That requires the Obelisk to expose an HTTP endpoint that can mutate the server (pull images, regenerate configs, restart containers). That endpoint must be locked down hard — anyone who can call it can run containers on your machine.

### The model: a cryptographic doorbell

Every developer has a local ED25519 keypair. Each Obelisk holds a list of public keys it trusts (`allowed_keys.json`), exactly like `~/.ssh/authorized_keys`. Every CLI request is individually signed; there are no sessions, tokens, or logins. The server verifies the signature, checks the key is on its list, and executes.

```
[ Dev A (laptop) ] ──signed HTTPS──► ┌────────── Obelisk server ──────────┐
                                     │  nginx (443, Let's Encrypt)        │
[ Dev B (laptop) ] ──signed HTTPS──► │    └─ /_obelisk/* ─► obelisk-agent │
                                     │                        │           │
                                     │         allowed_keys.json          │
                                     │         docker socket / .obelisk/  │
                                     └────────────────────────────────────┘
```

- **nginx** terminates TLS and proxies the reserved path `/_obelisk/` to the agent container over the internal docker network. The agent is never exposed directly.
- **obelisk-agent** is a tiny Go service that verifies signatures and executes a fixed set of operations (deploy, status, key management). It never executes arbitrary commands from the wire.
- Teammates get access by having an existing member push their public key to the server (`obelisk allow`).

### Why this model

- **Open-source friendly:** no central database, no account system to run, no vendor lock-in. Each Obelisk is sovereign.
- **No secrets in transit:** private keys never leave the laptop. Intercepting a request reveals nothing reusable (replay-protected).
- **Familiar:** it's the SSH trust model developers already understand.

---

## 2. Identity model

### Keypair

- **Algorithm:** ED25519 (modern, fast, small keys, misuse-resistant — same as current SSH defaults).
- **Location:** `~/.config/obelisk/id_ed25519` (private, mode `0600`) and `id_ed25519.pub` (public). Directory `~/.config/obelisk/` mode `0700`.
- **Generation:** automatic on first command that needs it, or explicitly via `obelisk identity --generate`. Never overwrite an existing key without `--force`.

### Public key string format

```
obk1_<base64(raw 32-byte ed25519 public key)> <optional comment>
```

Example: `obk1_MCowBQYDK2VwAyEA7s9Qx... nick@laptop`

The `obk1_` prefix versions the format (key type + encoding) so it can evolve.

### Fingerprint

SSH-style: `SHA256:<base64(sha256(raw public key))>`. Used everywhere a key is displayed or referenced (revocation, audit logs). Short, copy-pasteable, unambiguous.

---

## 3. Wire protocol (contract between obelisk-cli and obelisk-agent)

This section is the normative spec. Both repos implement exactly this; protocol changes bump the `v1` version string.

### Transport

- HTTPS only. The CLI refuses plain `http://` targets except `localhost`/`127.0.0.1` (local dev).
- TLS is terminated by the Obelisk's existing nginx + Let's Encrypt setup. The agent listens on plain HTTP **inside** the docker network only (e.g. port `9100`); nginx proxies `location /_obelisk/` to it.

### Request signing

Every request carries four headers:

| Header | Content |
|---|---|
| `X-Obelisk-Key` | Sender's public key (`obk1_...`, no comment) |
| `X-Obelisk-Timestamp` | Unix seconds at time of signing |
| `X-Obelisk-Nonce` | 16 random bytes, hex-encoded |
| `X-Obelisk-Signature` | base64 ED25519 signature over the canonical string |

**Canonical signing string** (newline-joined, exactly):

```
v1
{METHOD}
{PATH}
{timestamp}
{nonce}
{hex(sha256(request body))}
```

- `METHOD` uppercase (`GET`, `POST`, ...). `PATH` is the request path including the `/_obelisk` prefix as sent, no query string ambiguity — query params are not used; parameters go in the body or path.
- Empty body still hashes (`sha256("")`), so GETs are covered.
- Signing the body hash means the payload can't be swapped under a valid signature.

### Agent verification (in order, fail fast)

1. **Timestamp window:** reject if `|now - timestamp| > 60s` → `401 {"error": "stale_timestamp"}`. Tolerates reasonable clock skew, kills old captures.
2. **Replay check:** reject if `(key, nonce)` was seen within the last 120s (in-memory cache, ≥2× the timestamp window so a replayed request is always caught by one check or the other) → `401 {"error": "replay"}`.
3. **Key lookup:** reject if the key is not in `allowed_keys.json` → `403 {"error": "unknown_key"}`.
4. **Signature:** verify ED25519 signature over the recomputed canonical string → `401 {"error": "bad_signature"}` on failure.

All error responses are JSON: `{"error": "<code>", "message": "<human readable>"}`.

### Responses

JSON bodies, conventional status codes. The agent includes `X-Obelisk-Agent-Version` on every response so the CLI can warn on protocol drift.

---

## 4. Server-side agent (`obelisk-agent` repo)

### Shape

- Minimal Go HTTP server. Dependencies: standard library + `crypto/ed25519`. No framework.
- Ships as a Docker image (e.g. `ghcr.io/nickdill/obelisk-agent:<version>`), added as one more service in the Obelisk template's `docker-compose.yml` alongside nginx and certbot.
- Mounts:
  - `/var/run/docker.sock` — to pull images and restart containers
  - the Obelisk project directory — to invoke the existing `.obelisk/setup.sh` / `run.sh` flows
  - `./data/agent/` — persistent volume holding `allowed_keys.json` and the audit log, so **keys survive redeploys, updates, and restarts** (the persistence pain point from DECIDE.md)

### Fixed operations only

The agent maps endpoints to a hardcoded set of operations (run setup script, run deploy script, read compose status, edit the key file). Request bodies select *which module* or *which key*, never *what command*. There is no path from the wire to arbitrary execution.

### v1 API

All routes are under `/_obelisk` externally → `/v1/...` on the agent.

| Endpoint | Purpose |
|---|---|
| `GET /v1/ping` | Auth check + handshake. Returns agent version, server name, protocol version. |
| `GET /v1/status` | Module list with container states (running/exited/health). |
| `GET /v1/modules` | Current `obelisk.yml` module definitions on the server. |
| `POST /v1/deploy` | Trigger a deployment: pull latest image / git source for the named module(s), regenerate configs, restart, reload nginx. Body: `{"module": "api"}` or `{}` for all. |
| `GET /v1/keys` | List authorized keys (name, fingerprint, added_at, added_by). |
| `POST /v1/keys` | Authorize a key. Body: `{"key": "obk1_...", "name": "Teammate Bob"}`. |
| `DELETE /v1/keys/{fingerprint}` | Revoke a key. Refuses to delete the **last remaining** key (lockout guard). |

### `allowed_keys.json`

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

`version` exists so a `version: 2` with roles/scopes can be introduced later without breaking v1 agents or CLIs. Writes are atomic (write temp file + rename).

---

## 5. Bootstrap: getting the first key on the server

The trust chain has to start somewhere. It starts at provisioning, when you already control the server:

1. The Obelisk template's `.env` gains:
   ```
   OBELISK_AUTHORIZED_KEY="obk1_MCowBQYDK2VwAyEA... Nick (laptop)"
   ```
   (The dev gets this string from `obelisk identity` locally.)
2. `setup.sh` seeds `data/agent/allowed_keys.json` from `OBELISK_AUTHORIZED_KEY` **only if the file does not already exist**. Idempotent — re-running setup or redeploying never clobbers a live key list.
3. From then on, all key management happens over the signed API (`obelisk allow` / `obelisk revoke`). SSH is never required again for Obelisk operations.

---

## 6. CLI changes (`obelisk-cli`, this repo)

### New internal packages

| Package | Responsibility |
|---|---|
| `internal/identity` | Keypair generation, loading, fingerprinting, signing. Owns `~/.config/obelisk/id_ed25519`. |
| `internal/client` | Signed HTTP client: takes method/path/body, attaches the four auth headers, parses agent responses, surfaces friendly errors (stale clock, unknown key → "ask an admin to run `obelisk allow`"). |
| `internal/registry` | Local server registry at `~/.config/obelisk/servers.yml`: `{name: {url: https://...}}`. Add/list/remove/resolve. |

### Commands

| Command | Behavior |
|---|---|
| `obelisk identity` | Print public key string + fingerprint. Generates the keypair if missing. `--generate`/`--force` to rotate. |
| `obelisk server add <name> <url>` | Signed `GET /v1/ping` handshake first; on success, save to `servers.yml`. On `403 unknown_key`, print the dev's public key and instructions to send it to an admin. |
| `obelisk server list` | Show registered servers (name, url, last-seen status). |
| `obelisk server remove <name>` | Remove from local registry. |
| `obelisk allow <pubkey> --name <teammate> [--server <name>]` | `POST /v1/keys` — onboard a teammate. Prompts for server if multiple registered. |
| `obelisk revoke <fingerprint> [--server <name>]` | `DELETE /v1/keys/{fingerprint}`. |
| `obelisk list` | Fan out signed `GET /v1/status` to every registered server; render a combined table of servers → modules → states. |
| `obelisk deploy [--server <name>]` | Replace the current stub: resolve target server, `POST /v1/deploy` for the current project's module, stream/poll result. |

### Team onboarding flow (the multi-dev story)

1. Bob: `obelisk identity` → copies `obk1_...` string, sends it to Alice (Slack, etc.).
2. Alice: `obelisk allow obk1_... --name "Bob" --server prod`.
3. Bob: `obelisk server add prod https://obelisk.myteam.com` → handshake succeeds → Bob can now `obelisk list` / `deploy`.

---

## 7. Threat model

| Threat | Mitigation |
|---|---|
| Replay of a captured request | Timestamp window (±60s) + per-key nonce cache (120s). A request is unusable seconds after it's sent. |
| MITM / payload tampering | TLS required (CLI refuses non-localhost HTTP); body hash inside the signature. |
| Random internet actors hitting the endpoint | Every request must carry a valid signature from a listed key; unknown keys are rejected before any work happens. nginx rate-limits `/_obelisk/` as defense in depth. |
| Stolen laptop / leaked private key | File perms `0600`; revoke via `obelisk revoke` from any other authorized machine. *Future work:* passphrase-encrypted private key. |
| Compromised/departing teammate | Revoke their fingerprint. **Known v1 limitation:** all keys are equal, so any key can revoke any other — acceptable for small trusted teams, fixed by roles in a future `allowed_keys` v2. |
| Malicious deploy payloads | Agent executes only fixed, named operations; request bodies are parameters, never commands. |
| Tampering with `allowed_keys.json` | Requires host access, which is outside this trust boundary (host compromise = game over regardless). Atomic writes prevent corruption. |
| Lockout (revoking the last key) | Agent refuses to delete the final key; bootstrap re-seed path exists via SSH + `.env` as a recovery hatch. |

Every authenticated mutating operation is appended to an audit log (`data/agent/audit.log`): timestamp, fingerprint, operation, result.

---

## 8. Implementation roadmap

### Phase 1 — CLI identity foundation (this repo)

- Create `internal/identity`: ED25519 keygen, save/load with correct permissions, public key encoding (`obk1_`), fingerprinting, `Sign(method, path, timestamp, nonce, bodyHash)`.
- Add `obelisk identity` command.
- Unit tests: sign/verify round trip, key file permissions, format encode/decode.

**Done when:** `obelisk identity` prints a stable key + fingerprint across runs; tests pass.

### Phase 2 — Agent MVP (`obelisk-agent` repo)

- Scaffold the repo: Go module, HTTP server, Dockerfile, GitHub release workflow publishing the image.
- Implement the verification middleware (section 3, all four checks) and `allowed_keys.json` store with atomic writes.
- Implement `GET /v1/ping` and `GET /v1/status`.
- Unit tests: each rejection path (stale timestamp, replayed nonce, unknown key, tampered body/signature) plus the happy path.

**Done when:** agent runs locally in Docker, a hand-signed request gets `200`, all tamper cases are rejected with the right error codes.

### Phase 3 — Template integration & bootstrap (Obelisk template repo)

- Add the `obelisk-agent` service to the template's `docker-compose.yml` (docker socket + project dir + `data/agent/` mounts).
- Add the `location /_obelisk/` proxy block to the nginx templates (both prod and local variants).
- Add `OBELISK_AUTHORIZED_KEY` to `.env.example`; extend `setup.sh` to seed `allowed_keys.json` iff missing.

**Done when:** a freshly provisioned Obelisk answers a signed `/v1/ping` from the key named in `.env`, over HTTPS through nginx.

### Phase 4 — Server registry & connectivity (this repo)

- Create `internal/client` (signed HTTP) and `internal/registry` (`servers.yml`).
- Add `obelisk server add/list/remove` with the ping handshake and the "send your key to an admin" UX on 403.
- Implement `obelisk list` fanning out to all registered servers.

**Done when:** `obelisk server add` + `obelisk list` work against a live agent; unknown-key UX is friendly.

### Phase 5 — Key management / team onboarding

- Agent: `GET/POST /v1/keys`, `DELETE /v1/keys/{fingerprint}`, last-key lockout guard, audit log.
- CLI: `obelisk allow`, `obelisk revoke`.

**Done when:** the full Bob-onboarding flow (section 6) works end-to-end; revoked key is rejected on its next request.

### Phase 6 — Authenticated deploy

- Agent: `POST /v1/deploy` — invoke the template's setup/run scripts for the named module (pull image / refresh git source, regenerate compose + nginx configs, restart, reload).
- CLI: wire `obelisk deploy` (currently a stub in `cmd/deploy.go`) to resolve the project's module from `obelisk.yml` (`internal/config`), pick the target server, call deploy, and report the result.

**Done when:** editing a module, running `obelisk deploy`, and seeing the change live on a test Obelisk works with no SSH.

### Phase 7 — Hardening & polish

- nginx rate limiting on `/_obelisk/`.
- Clock-skew detection: agent returns its own time on 401 stale_timestamp so the CLI can say "your clock is off by Ns".
- Key rotation docs (`identity --generate --force` + re-`allow` from a second device).
- Document the recovery hatch (re-seed via SSH) and the v2 roles plan.

---

## 9. Verification strategy

- **Unit (both repos):** canonical-string construction matches byte-for-byte across CLI and agent (golden test vectors checked into both repos); sign/verify round trip; every middleware rejection path.
- **Integration:** docker-compose file in `obelisk-agent` that runs the agent + a fake project dir; CLI integration tests run `server add` → `list` → `allow` → `revoke` → `deploy` against it on `localhost` (HTTP allowed for localhost only).
- **End-to-end:** provision a real test Obelisk with the updated template, seed a key via `.env`, then exercise the full flow from a laptop: handshake, onboard a second key, deploy a module update, revoke the second key and confirm rejection.

---

## 10. Explicitly out of scope (v1)

- Megalisk accounts, `obelisk login`, hosted control plane, billing — not designed for here at all.
- Key roles/permissions and per-module scoping (reserved for `allowed_keys.json` v2).
- Server provisioning (`obelisk deploy new` / EC2 creation).
- Passphrase-encrypted private keys (noted as future hardening).
