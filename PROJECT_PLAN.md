# PROJECT_PLAN — obelisk-cli

What's left to build, fix, and clean up. Compiled 2026-06-10 from the codebase,
`concerns_20260610.md`, `work_notes.md`, `CLI_AUTHENTICATION_PLAN.md` (Phase 7),
`plans/PLAN_SERVER_PROVISIONING.md`, and the unreleased CHANGELOG entries.

---

## Where the project stands

The core loop is built and working: identity (`internal/identity` + `obelisk identity`),
signed HTTP (`internal/client`), server registry (`internal/registry` +
`obelisk server add/list/remove`), team onboarding (`allow`/`revoke`), fan-out
`obelisk list`, streaming `obelisk deploy`, and local-dev commands
(`new`/`init`/`dev`/`down`/`logs`/`debug`/`status`/`uninstall`). `go build` and
`go test` pass.

What remains falls into six workstreams, ordered below by priority.

---

## 1. Fix known bugs (blocks release of current work)

Six issues from the 2026-06-10 code review (`concerns_20260610.md`) plus one from
`work_notes.md`. All are in uncommitted or recently committed code.

- [ ] **1.1** `.gitignore` check uses `strings.Contains(".env")` — matches `.env.local`,
      `.envrc`, comments; bare `.env` never gets appended and the generated `.env` can be
      committed. Match exact trimmed lines instead. (`cmd/init.go:90`, security)
- [ ] **1.2** `.gitignore` write errors are discarded; "update .gitignore" prints even on
      failure. Use `os.WriteFile` and check the error. (`cmd/init.go:93`)
- [ ] **1.3** Non-purge `obelisk uninstall` silently changed meaning — it now leaves
      `.obelisk/` and `docker-compose.yml` behind while the flag text implies otherwise.
      Decide: restore old behavior or keep new behavior and fix the `--purge` description
      plus add "what was NOT removed" messaging. (`cmd/uninstall.go:70`)
- [ ] **1.4** `isServer` check ignores `obelisk.local.yml`, so a valid local-only project
      fails uninstall with "no Obelisk project found". (`cmd/uninstall.go:36`)
- [ ] **1.5** Old-format module projects (`obelisk.yml` with `type: module`, no
      `obelisk.module.yml`) are misidentified as servers by both `uninstall` and `status`.
      Add a migration check on `type: module`. (`cmd/uninstall.go:35`, `internal/config`)
- [ ] **1.6** `checkGenerateComposeStale` swallows all read errors, not just
      `os.IsNotExist`, hiding permission/I-O failures behind a cryptic shell error later.
      (`cmd/dev.go:64`)
- [ ] **1.7** `obelisk dev` fails when `obelisk.yml` contains an `image:` for a module
      that isn't deployed/pulled locally (from `work_notes.md`). Reproduce, then decide:
      skip unpullable images in local dev, or require `obelisk.local.yml` overrides with
      a clear error pointing at the offending module.

**Done when:** all six review items are fixed with regression tests where practical,
the `dev` bug has a decided behavior, and the pending changes are committed.

---

## 2. Test coverage (currently only `internal/identity` has tests)

The auth plan §9 already defines the strategy; most of it is unimplemented on the CLI side.

- [ ] **2.1** Unit tests for `internal/config` — `Load`, `LoadModule`, `IsModule`,
      local-override precedence, old-format `type: module` detection (ties into 1.5).
- [ ] **2.2** Unit tests for `internal/registry` — add/remove/list/resolve round-trip,
      single-server auto-resolve, ambiguous-server error.
- [ ] **2.3** Unit tests for `internal/client` — header construction against the golden
      vectors shared with `obelisk-agent`, HTTPS enforcement for non-localhost,
      unknown-key and stale-timestamp error rendering.
- [ ] **2.4** Unit tests for `cmd` helpers — `streamDeploy` exit-code parsing (including
      the no-trailing-JSON failure path), `dockerComposePS` array vs NDJSON parsing.
- [ ] **2.5** Integration suite: run the real `obelisk-agent` (compose file lives in
      `../obelisk-agent`) and exercise `server add` → `list` → `allow` → `revoke` →
      `deploy` against localhost.

**Done when:** every internal package has tests, and the integration loop runs locally
with one command.

---

## 3. `obelisk publish` — the last stubbed command

`cmd/publish.go` still prints "coming soon". Scope (from CLAUDE.md and vision.md):
build the module's Docker image and push it to ECR or any registry.

- [ ] **3.1** Design pass: where the image name/tag comes from (`obelisk.yml` `image:`
      field is the obvious source), tagging scheme (latest vs git SHA), and ECR auth
      (shell out to `docker` + `aws ecr get-login-password`, or document prerequisites).
- [ ] **3.2** Implement: `docker build` + `docker push` with streamed output, `--tag`
      override, clear errors when `image:` is absent or the registry rejects the push.
- [ ] **3.3** Wire into the deploy story: document (or implement) `publish` → `deploy`
      as the standard ship flow for image-based modules.

**Done when:** from a module repo, `obelisk publish && obelisk deploy` updates a live
server with no SSH and no manual docker commands.

---

## 4. Server provisioning (`plans/PLAN_SERVER_PROVISIONING.md`)

The biggest remaining feature: `obelisk server new` creates an EC2 instance via
cloud-init that boots into a pingable Obelisk. The plan is fully designed; its six
phases are the work items. Auth-plan dependencies are already met.

- [ ] **4.1** Phase 1 — `internal/provision` Provider interface + ServerSpec/Server
      types; extend `internal/registry` schema with provider metadata
      (`provider`, `instance_id`, `region`).
- [ ] **4.2** Phase 2 — AWS provider: aws-sdk-go-v2, STS preflight, security-group
      create-or-reuse, SSM AMI lookup, tagged launch, tag-checked destroy.
      (First new external dependency — note in CLAUDE.md when added.)
- [ ] **4.3** Phase 3 — embedded cloud-init template + renderer (template release URL,
      server name, creator's public key); golden-file tests; **resolve the open
      first-contact TLS question** (self-signed bootstrap cert vs HTTP-on-IP for the
      initial ping) against the template's nginx config.
- [ ] **4.4** Phase 4 — `obelisk server new / destroy / status`: wizard, cost
      confirmation, signed-ping readiness polling, failure cleanup, registry write.
- [ ] **4.5** Phase 5 — empty-registry fallback in `obelisk deploy`: TTY-gated prompt
      into the wizard, then continue the deploy; non-TTY fails with guidance.
- [ ] **4.6** Phase 6 — fast-follows: Elastic IP, Route 53 `--domain` automation,
      per-size cost table, docs.

**Done when:** `obelisk server new prod` goes from nothing to a registered, pingable
server in one command on a sandbox AWS account, and `destroy` leaves no orphaned
resources.

---

## 5. Auth hardening & polish (auth plan Phase 7 remainder)

Most of Phase 7 is done (clock-skew messaging ships in `internal/client`). Remaining:

- [ ] **5.1** nginx rate limiting on `/_obelisk/` — template/agent-side change; track
      here, implement in `obelisk-template` / `../obelisk-agent`.
- [ ] **5.2** Key rotation docs: `identity --force` + re-`allow` from a second device.
- [ ] **5.3** Document the recovery hatch (re-seed `allowed_keys.json` via SSH) and the
      v2 roles plan.
- [ ] **5.4** End-to-end verification on a real Obelisk: handshake, onboard a second
      key, deploy, revoke, confirm rejection (auth plan §9).

---

## 6. Repo & docs hygiene

- [ ] **6.1** Commit the pending working-tree changes (CHANGELOG, README, USAGE,
      `docs/QUICKSTART_AUTH.md`, `cmd/list.go`) — after workstream 1, since the review
      findings touch the same code.
- [ ] **6.2** Update CLAUDE.md: status table says `obelisk status` is "later" but it's
      implemented; project-structure tree still lists `deploy.go`/`status.go` as STUB
      and `internal/identity|client|registry` as TODO; `obelisk.module.yml` naming
      change should be reflected in the schema section.
- [ ] **6.3** Consolidate planning docs: ~15 markdown files sit at the repo root
      (`TASK_*.md`, `concerns_*.md`, `work_notes.md`, `DECIDE.md`, `SUMMARY.md`,
      `PROMPT_HISTORY.md`, marketing notes…). Move historical/scratch docs into
      `plans/` or a `notes/` dir; keep README, USAGE, CHANGELOG, CLAUDE.md, and the
      normative specs at root. Delete `concerns_20260610.md` and the relevant
      `work_notes.md` line once workstream 1 lands.
- [ ] **6.4** First tagged release: `build_versioning_guide.md` exists but there's no
      tag; cut `v0.1.0` once workstreams 1–2 land, move CHANGELOG `[Unreleased]` under
      it, and verify `install.sh` / `update.sh` against the tag.

---

## Explicitly deferred (tracked, not planned)

- Megalisk hosted provider (`--provider megalisk`, accounts, billing) — interface frozen
  in the provisioning plan §6; build when demand exists.
- Key roles / per-module permission scoping (`allowed_keys.json` v2).
- Passphrase-encrypted private keys.
- Additional cloud providers (DigitalOcean, Hetzner).
- Admin web module for managing modules from a browser (DECIDE.md idea).

---

## Suggested order

1 (bug fixes) → 6.1/6.2 (commit + doc truth) → 2 (tests) → 6.4 (v0.1.0 release) →
3 (publish) → 4 (provisioning, phases in order) → 5 (hardening) → 6.3 (doc cleanup,
anytime).

Rationale: the uncommitted work has known security-adjacent bugs, so fix-then-commit
comes first. Tests before release. Publish before provisioning because it completes
the ship loop for users who already have servers, and is far smaller. Provisioning is
the long pole and has its own internal ordering.
