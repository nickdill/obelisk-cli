# Server Provisioning Plan

Design for creating Obelisk servers from the CLI, answering: *what happens when a user runs `obelisk deploy` and has no server?*

This document builds directly on `CLI_AUTHENTICATION_PLAN.md`. The definition of a successfully provisioned server is borrowed from it: **a machine that answers a signed `GET /v1/ping` from the creator's key.** Provisioning is everything required to get from "nothing" to that state.

**Decisions locked in:**

| Decision | Choice |
|---|---|
| Strategy | Hybrid. v1 is BYO-cloud: the CLI provisions into the **user's own cloud account** with their credentials — no accounts, no login. The future Megalisk hosted service is designed as *just another provider* behind the same interface (API sketched here, not built). |
| First provider | AWS EC2 — matches the existing stack (current Obelisks run on EC2; vision.md plans ECR/Route 53 integration). |
| Server configuration | Cloud-init user-data at instance creation. No SSH orchestration layer, no AMI build pipeline. |
| Empty-registry UX | `obelisk deploy` with no servers prompts: "No Obelisk servers found. Create one now?" and walks through creation interactively. |

---

## 1. Strategic rationale: why hybrid, why not a hosting product first

The fork was: (a) CLI drives the user's own cloud account, (b) build a separate hosted product (Megalisk) exposing a provisioning API, or (c) both behind one abstraction.

**Hybrid, BYO first** wins because:

- **It ships without a SaaS.** A hosted product means accounts, billing, abuse handling, and 24/7 ops before the first user deploys anything. BYO needs none of that — the user's AWS bill is their own.
- **It preserves the open-source trust position.** The auth system was deliberately designed with no central login (CLI_AUTHENTICATION_PLAN.md). Requiring a Megalisk account to get a server would undercut that immediately.
- **It doesn't foreclose the business.** The `Provider` interface (§3) means Megalisk later becomes `--provider megalisk` — same commands, same UX, serving users who *don't* have a cloud account and are happy to pay for that. That's the revenue path, added when there's demand, not before.

**How Megalisk coexists with "no accounts":** a Megalisk account authenticates *provisioning and billing only*. The server it creates is a normal Obelisk that trusts the user's ED25519 public key (passed in the create request). Day-to-day access — deploy, list, status, key management — stays on the decentralized keypair model. Megalisk never holds a private key and is not in the request path after creation.

---

## 2. The user experience

### Explicit path

```
$ obelisk server new prod
  Provider:  aws (default)
  Region:    us-east-2
  Size:      t3.small  (~$15/mo)
  Create this server? This will create billable AWS resources. (y/N) y

  ⏳ Launching EC2 instance...           i-0abc123
  ⏳ Waiting for Obelisk to come online (this takes ~3 minutes)...
  ✅ prod is ready at 3.18.42.7

  Registered as 'prod'. Point a domain at 3.18.42.7, then run:
    obelisk deploy --server prod
```

### Fallback path (the original question)

```
$ obelisk deploy
  No Obelisk servers found.
  Create one now? (Y/n) y
  [ ... server new wizard ... ]
  ✅ prod is ready. Continuing deploy...
```

The prompt only appears in interactive terminals; in CI/non-TTY contexts `obelisk deploy` fails with the `obelisk server new` instruction instead. A deploy command never silently creates infrastructure.

---

## 3. Provider abstraction (`internal/provision`)

```go
type Provider interface {
    Name() string
    Validate(ctx context.Context) error                          // credentials / connectivity preflight
    Create(ctx context.Context, spec ServerSpec) (*Server, error) // blocks until instance exists, returns ID + public IP
    Destroy(ctx context.Context, serverID string) error
    Status(ctx context.Context, serverID string) (State, error)   // provider-level state (running/stopped/terminated)
}

type ServerSpec struct {
    Name     string
    Region   string
    Size     string            // provider-native size token (e.g. "t3.small")
    UserData string            // rendered cloud-init script
    Tags     map[string]string
}

type Server struct {
    ID       string // provider-native ID (e.g. EC2 instance ID)
    PublicIP string
}
```

- v1 ships one implementation: `aws`. DigitalOcean/Hetzner fit the same interface later; `megalisk` is the hosted implementation (§6).
- **Registry extension:** entries in `~/.config/obelisk/servers.yml` (from CLI_AUTHENTICATION_PLAN.md §6) gain optional provisioning metadata:

  ```yaml
  prod:
    url: https://3.18.42.7        # or domain once pointed
    provider: aws                  # absent for manually-added servers
    instance_id: i-0abc123
    region: us-east-2
  ```

  Manually-added servers (`obelisk server add`) have no provider metadata; `destroy` and provider-level `status` refuse to operate on them.

---

## 4. AWS provider (v1)

### Credentials

Standard AWS SDK credential chain — env vars, `~/.aws/credentials`, SSO — with a `--profile` flag passed through. The CLI never stores AWS secrets of its own. `Validate()` runs `sts:GetCallerIdentity` as a preflight so credential problems surface before anything is created.

### Resources created

| Resource | Detail |
|---|---|
| EC2 instance | Default `t3.small` (2 vCPU / 2GB — docker + nginx + a few modules with headroom; `--size t3.micro` for free-tier toes). Ubuntu 24.04 LTS, AMI resolved at runtime via the SSM public parameter (no hardcoded AMI IDs that rot per region). |
| Security group | `obelisk` (created if missing, reused if present): inbound 80/443 from anywhere. Port 22 opened **only** if `--ssh-key <name>` is passed (debugging escape hatch; also enables the auth plan's bootstrap-recovery path). |
| Storage | 25GB gp3 root volume. |
| Network | Default VPC, auto-assigned public IP. |
| Tags | `obelisk:managed=true`, `obelisk:name=<name>` on every created resource. |

### Guardrails (creating billable things demands care)

- **Cost up front:** the confirmation prompt shows the instance size and approximate monthly cost before anything is created. `--yes` skips for scripting.
- **Tagged-only destroy:** `obelisk server destroy` verifies the instance carries `obelisk:managed=true` before terminating, and requires re-typing the server name. It will never touch untagged infrastructure.
- **Failure cleanup:** if provisioning fails partway (instance launched but never becomes ready), the CLI offers to terminate the orphan rather than leaving a billing surprise.

### IP and DNS

v1 prints the public IP with "point your domain at this address" instructions. Two fast-follows (not v1): Elastic IP allocation (so the IP survives stop/start) and Route 53 record automation (already on vision.md's roadmap).

---

## 5. Cloud-init bootstrap

A cloud-init script template embedded in the CLI binary (`go:embed`), rendered per-launch with three parameters:

1. **Obelisk template release** — pinned version URL of the Obelisk template tarball
2. **Server name**
3. **Creator's public key** — read from `~/.config/obelisk/id_ed25519.pub` via `internal/identity`

The rendered script, executed by the instance on first boot:

```
1. apt install docker.io + compose plugin
2. fetch + unpack the Obelisk template release into /opt/obelisk
3. write .env  — including OBELISK_AUTHORIZED_KEY="obk1_... <name>"
4. run .obelisk/setup.sh   (seeds allowed_keys.json per auth plan §5)
5. run .obelisk/run.sh     (nginx + certbot + obelisk-agent come up)
```

This is exactly the bootstrap path from CLI_AUTHENTICATION_PLAN.md §5 — provisioning adds nothing new to the trust model; it just automates writing the `.env`.

### Readiness

After `Create()` returns an IP, the CLI polls **signed `GET /v1/ping`** (the standard handshake) every 10s with a ~5 minute timeout. First successful ping ⇒ the server is provisioned, trusted, and proven to trust *you* — then it's written to the registry. On timeout, the CLI reports how to inspect the instance (console output, the `--ssh-key` hatch) and offers cleanup.

Until a domain is pointed and certs issue, the agent is reachable by IP; the CLI tolerates the certificate situation for the initial ping (self-signed bootstrap cert or HTTP-on-IP for first contact only — to be finalized in implementation against the template's nginx behavior, and noted as the one open question of this design).

---

## 6. Megalisk hosted provider (future — sketched, not built)

When users without cloud accounts show up, Megalisk implements the same `Provider` interface, backed by a REST API:

| Endpoint | Purpose |
|---|---|
| `POST /v1/servers` | Create. Body: `{name, region, size, public_key: "obk1_..."}`. Returns server ID + endpoint. |
| `GET /v1/servers/{id}` | Status. |
| `DELETE /v1/servers/{id}` | Destroy (stops billing). |

- Authenticated by a Megalisk account (`obelisk login`, device flow or API token) — used **only** here, for provisioning/billing.
- The created server is a stock Obelisk seeded with the user's public key; from the first ping onward it's indistinguishable from a BYO server. Keypair auth governs all ongoing access; Megalisk holds no private keys.
- UX: `obelisk server new --provider megalisk`, and the no-server deploy prompt can offer it as the "I don't have an AWS account" option.

Nothing in v1 builds this; the interface and API shape are frozen here so it slots in without reworking the CLI.

---

## 7. CLI commands

| Command | Behavior |
|---|---|
| `obelisk server new [name]` | Flags: `--provider` (default `aws`), `--region`, `--size`, `--profile`, `--ssh-key`, `--yes`. Interactive wizard for anything omitted. Validate → confirm cost → Create → poll signed ping → register. |
| `obelisk server destroy <name>` | Tagged-only, name-confirmation, terminates via provider, removes from registry. |
| `obelisk server status <name>` | Provider instance state + agent ping result side by side. |
| `obelisk deploy` (no servers) | Interactive TTY: prompt to run the `server new` wizard, then continue the deploy. Non-TTY: fail with guidance. |

These join the `server add/list/remove` commands from the auth plan under one `server` command group.

---

## 8. Implementation roadmap

> Ordering note: Phases 3–4 depend on the auth plan's Phase 1 (identity), Phase 2 (published agent image), and Phase 3 (template integration). Provisioning Phases 1–2 can proceed in parallel with auth work.

### Phase 1 — Provider abstraction & registry
`internal/provision` interface + types; extend `internal/registry` schema with provider metadata.
**Done when:** registry round-trips provisioned and manual entries; interface compiles with a stub provider used in tests.

### Phase 2 — AWS provider
aws-sdk-go-v2 dependency; `Validate` (STS), SG create-or-reuse, SSM AMI lookup, launch with user-data + tags, `Status`, tag-checked `Destroy`.
**Done when:** against a sandbox account, a Go test (build-tagged, manual) launches, statuses, and terminates a tagged instance.

### Phase 3 — Cloud-init template
Embedded template + renderer (version URL, name, public key); golden-file tests; resolve the first-contact TLS question (§5) against the template's nginx config.
**Done when:** rendered user-data, applied to a manually-launched instance, produces a server that answers a signed ping.

### Phase 4 — `obelisk server new / destroy / status`
Wizard, cost confirmation, readiness polling, failure cleanup, registry write.
**Done when:** `obelisk server new prod` goes from nothing to a registered, pingable server in one command; `destroy` cleans it up completely.

### Phase 5 — Deploy fallback
Empty-registry detection in `obelisk deploy` (cmd/deploy.go), TTY-gated prompt into the wizard, continue deploy after.
**Done when:** fresh machine + `obelisk deploy` reaches a successful deploy via the interactive path; non-TTY fails with guidance.

### Phase 6 — Polish & fast-follows
Elastic IP option, Route 53 record automation (`--domain` flag), cost table per size, docs.

### Future — Megalisk provider
Build the hosted API of §6 when demand justifies operating it.

---

## 9. Verification strategy

- **Unit:** cloud-init rendering (golden files), registry schema round-trip, AWS provider against mocked SDK client interfaces, tag-guard on destroy.
- **Integration (sandbox AWS account):** full lifecycle — `server new` → instance boots → signed ping → `obelisk deploy` to it → `server destroy` → assert no orphaned instances, SGs, or volumes remain.
- **UX walkthrough:** empty registry + `obelisk deploy` interactive path end-to-end; non-TTY behavior in CI.

---

## 10. Out of scope (v1)

- Building the Megalisk hosted product (sketched in §6 only)
- Additional providers (DigitalOcean, Hetzner — interface accommodates them)
- Route 53 / Elastic IP automation (fast-follows in Phase 6)
- Prebuilt AMI/image pipeline
- Server resize, migration, or multi-server orchestration
