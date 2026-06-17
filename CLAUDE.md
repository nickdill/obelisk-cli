# obelisk-cli

## What this is

The `obelisk` CLI manages the full lifecycle of Obelisk projects and modules — from local development through production deployment. An **Obelisk** is a self-hosted Docker Compose + nginx stack that runs multiple apps (modules) on one server. This CLI is the primary tool for creating, running, and deploying them.

---

## Ecosystem

```
obelisk-cli (this repo)          — developer CLI
obelisk-agent (../obelisk-agent) — HTTP agent that runs on every deployed Obelisk server
obelisk                          — base Docker Compose project scaffolded by `obelisk init`
```

The CLI communicates with deployed servers exclusively through `obelisk-agent` over signed HTTPS. No SSH required after initial bootstrap.

---

## Tech stack

- **Language:** Go 1.22
- **CLI framework:** `github.com/spf13/cobra`
- **Config parsing:** `gopkg.in/yaml.v3`
- **Module path:** `github.com/nickdill/obelisk`
- **No other external dependencies** (auth will use stdlib `crypto/ed25519`)

---

## Project structure

```
main.go                    calls cmd.Execute()
cmd/
  root.go                  banner, version, command registration
  httpclient.go            shared HTTP client constructor (timeout + dial config)
  template.go     ✅       shared template download logic; templateRef const controls branch/tag
  new.go          ✅       downloads obelisk-template tarball, scaffolds new project dir
  init.go         ✅       server mode: downloads obelisk-template into cwd; module mode: writes two hardcoded files
  dev.go          ✅       runs .obelisk/dev.sh (or run.sh fallback); --build flag pre-builds images
  build.go        ✅       runs .obelisk/build.sh (module compilation)
  run.go          ✅       ensures Swarm manager (auto-inits if inactive), checks yq dep, then runs .obelisk/run.sh
  stop.go         ✅       runs .obelisk/stop.sh (stops all services)
  down.go         ✅       docker stack rm obelisk (Swarm)
  logs.go         ✅       docker service logs <module> (Swarm; requires exactly one module name)
  debug.go        ✅       prints active obelisk.yml / obelisk.local.yml
  update.go       ✅       self-updater; downloads correct binary from GitHub Releases; accepts optional version arg
  uninstall.go    ✅       removes obelisk-managed files from cwd (type-aware)
  identity.go     ✅       keypair display + generation
  allow.go        ✅       POST /v1/keys — authorize a teammate's public key
  revoke.go       ✅       DELETE /v1/keys/{fingerprint}
  server.go       ✅       server add / list / remove subcommands
  deploy.go       ✅       POST /v1/deploy; streams output; sends git sha/branch/tag metadata
  status.go       ✅       local project status + docker stack services (Swarm replicas)
  scale.go        ✅       POST /v1/scale — set replica count for a module
  list.go         ✅       GET /v1/status fan-out across all registered servers; combined table output
  publish.go      STUB     prints "coming soon" — needs full implementation

internal/
  config/
    config.go     ✅       loads obelisk.yml / obelisk.local.yml; Config and Module structs
  identity/       ✅       ED25519 keypair gen/load/sign, obk1_ encoding, fingerprinting
  client/         ✅       signed HTTP client for talking to obelisk-agent
  registry/       ✅       local server registry (~/.config/obelisk/servers.yml)
```

---

## obelisk.yml schema

Both server projects and module repos use `obelisk.yml`. The `type:` field distinguishes them.

**Server project** (`type: server`):
```yaml
version: "0.1"
name: "my-project"
type: server

modules:
  api:
    image: 123456789.dkr.ecr.us-east-2.amazonaws.com/my-api:latest
    port: 3000
    domain: api.example.com
    env:
      DATABASE_URL: postgres://...

  web:
    git_source: https://github.com/myorg/web-app
    port: 8080
    domain: example.com

  docs:
    git_source: ../docs-site   # local path (obelisk.local.yml only)
    port: 4000
    domain: docs.localhost
    type: static
```

**Module repo** (`type: module`):
```yaml
version: "0.1"
name: "my-api"
type: module
port: 3000
```

`obelisk.local.yml` overrides `obelisk.yml` when present in server projects (local dev paths, no SSL).

---

## Auth model

The CLI and `obelisk-agent` use an ED25519 signed-request protocol — no sessions, no central login. Full spec is in `CLI_AUTHENTICATION_PLAN.md`.

### Public key format

```
obk1_<base64(raw 32-byte ed25519 public key)> <optional comment>
```

Example: `obk1_MCowBQYDK2VwAyEA... nick@laptop`

Keys live at `~/.config/obelisk/id_ed25519` (private, mode 0600) and `id_ed25519.pub`.

### Fingerprint format

`SHA256:<base64url-nopad(sha256(raw public key bytes))>` — URL-safe alphabet, no padding, so a fingerprint can appear verbatim in a path segment (`DELETE /v1/keys/{fingerprint}`).

### Request signing (every CLI → agent call)

Four headers attached by `internal/client`:

| Header | Content |
|---|---|
| `X-Obelisk-Key` | Full public key string (`obk1_...`) |
| `X-Obelisk-Timestamp` | Unix seconds at signing time |
| `X-Obelisk-Nonce` | 16 random bytes, hex-encoded |
| `X-Obelisk-Signature` | base64 ED25519 sig over canonical string |

Canonical string (newline-joined):
```
v1
{METHOD}
{PATH}
{timestamp}
{nonce}
{hex(sha256(request body))}
```

`PATH` is the **agent-side escaped path** (`/v1/...`), never the `/_obelisk` proxy prefix — proxy-agnostic. Use `url.EscapedPath()` equivalent when constructing it.

---

## Internal packages

### `internal/identity`

Owns the local keypair at `~/.config/obelisk/`.

```go
Generate() error                       // creates id_ed25519 + id_ed25519.pub
Load() (ed25519.PrivateKey, error)
PublicKeyString() (string, error)      // returns "obk1_<base64>"
Fingerprint() (string, error)          // returns "SHA256:..."
Sign(method, path, timestamp, nonce, bodyHash string) (string, error)  // returns base64 sig
```

Never overwrite an existing key without an explicit `--force` flag.

### `internal/client`

Signed HTTP client. Reads the local keypair, attaches the four auth headers, surfaces friendly errors.

```go
type Client struct { BaseURL string }
func (c *Client) Get(path string) (*http.Response, error)
func (c *Client) Post(path string, body any) (*http.Response, error)
func (c *Client) Delete(path string) (*http.Response, error)
```

On `403 unknown_key`: print the dev's public key and instructions to send it to an admin.
On `401 stale_timestamp`: show the clock skew from the agent's response.

### `internal/registry`

Local server registry at `~/.config/obelisk/servers.yml`.

```yaml
servers:
  prod:
    url: https://prod.example.com
    last_seen: 2026-06-09T20:00:00Z
  staging:
    url: https://staging.example.com
```

```go
Add(name, url string) error
Remove(name string) error
List() ([]Server, error)
Resolve(name string) (Server, error)   // "" → first server if only one registered
```

---

## Commands

### `obelisk identity`

Print the local public key + fingerprint. Generate keypair on first run.

```
$ obelisk identity
Public key:  obk1_MCowBQY...
Fingerprint: SHA256:abc123...

Send your public key to a server admin to get access.
```

Flags: `--generate` / `--force` to rotate.

### `obelisk server add <name> <url>`

1. Perform a signed `GET /v1/ping` handshake
2. On success: save to `servers.yml`, print server name + agent version
3. On `403 unknown_key`: print public key + "ask an admin to run `obelisk allow`"

### `obelisk server list`

Table of registered servers: name, URL, last-seen status.

### `obelisk server remove <name>`

Remove from local `servers.yml`.

### `obelisk allow <pubkey> [--name <label>] [--server <name>]`

`POST /v1/keys` to the target server. Prompts for server if multiple registered and `--server` not given.

### `obelisk revoke <fingerprint> [--server <name>]`

`DELETE /v1/keys/{fingerprint}` to the target server.

### `obelisk list`

Fan out `GET /v1/status` to every registered server. Render a combined table:

```
SERVER    URL                       MODULE   STATE    HEALTH
prod      https://obelisk.myteam.com  api      running  healthy
prod      https://obelisk.myteam.com  web      running
staging   https://staging.example.com  api      exited
```

### `obelisk deploy [--server <name>]`

1. Load `obelisk.yml` via `internal/config` to get the current module name
2. Resolve target server from `internal/registry` (auto-selects when only one registered)
3. `POST /v1/deploy` with `{"module": "<name>", "sha": "...", "branch": "...", "tag": "..."}` — git metadata fields are omitted when not available (e.g., detached HEAD, no tag)
4. Stream the response body (script output) to stdout
5. Parse the trailing `{"exit_code": N}` line; exit non-zero on failure

### `obelisk scale <module> <replicas> [--server <name>]`

`POST /v1/scale` with `{"module": "<name>", "replicas": N}`. Adjusts the Docker Swarm replica count for the named service.

---

## Team onboarding flow

```
Bob:   obelisk identity             → copies obk1_... string, sends to Alice
Alice: obelisk allow obk1_... --name "Bob" --server prod
Bob:   obelisk server add prod https://obelisk.myteam.com
Bob:   obelisk deploy              → works
```

---

## Build & run

```bash
go build -ldflags "-X cmd.version=dev" -o obelisk .
./obelisk --help
```

The version var is in `cmd/root.go` as `var version = "dev"`.

---

## Key planning docs in this repo

| File | Purpose |
|---|---|
| `CLI_AUTHENTICATION_PLAN.md` | Normative auth spec — wire protocol, identity model, threat model, build roadmap |
| `OBELISK_AGENT_PLAN.md` | Spec for the companion `obelisk-agent` service (already built at `../obelisk-agent`) |
| `CLI_SCOPE.md` | Original scope discussion and architecture analysis |
| `DECIDE.md` | Command surface decisions and open questions |
| `vision.md` | Product vision and `obelisk.yml` schema reference |

---

## What's done vs what's next

| Area | Status |
|---|---|
| `obelisk new` — scaffold from template | ✅ |
| `obelisk init` — create obelisk.yml + scripts | ✅ |
| `obelisk dev` / `down` / `logs` / `debug` | ✅ |
| `obelisk build` / `run` / `stop` — module build + production lifecycle | ✅ |
| `obelisk update [version]` — self-updater from GitHub Releases | ✅ |
| `internal/config` — yaml loading | ✅ |
| `obelisk-agent` (separate repo) | ✅ built at `../obelisk-agent` |
| `internal/identity` — keypair + signing | ✅ |
| `obelisk identity` command | ✅ |
| `internal/client` — signed HTTP | ✅ |
| `internal/registry` — server registry | ✅ |
| `obelisk server add/list/remove` | ✅ |
| `obelisk allow` / `obelisk revoke` | ✅ |
| `obelisk list` — fan-out status | ✅ |
| `obelisk deploy` — replace stub | ✅ |
| `obelisk status` — local + Swarm status | ✅ |
| `obelisk scale` — set replica count | ✅ |
| `obelisk publish` — build + push images | later |
