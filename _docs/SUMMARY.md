# Project Status Summary

*Last verified: 2026-06-09 — every claim below checked against the actual code, not docs.*

## TL;DR

The local development loop works end to end (`new` → `init` → `dev` → `logs` → `down`). Everything remote is not built yet: `deploy`, `status`, and `publish` are stubs that print "coming soon." The companion `obelisk-agent` (at `../obelisk-agent`) is fully written and compiles, but a code review (`FABLE_ADVICE.md` in that repo) found 4 critical bugs that will break it on first contact with the CLI — fix those before building the CLI side of auth. The auth design itself is settled and documented; no code for it exists in this repo yet.

---

## Repo & branch map

| Repo | Where | State |
|---|---|---|
| **obelisk-cli** (this repo) | `planning` branch | `main` has only the initial commit. `planning` adds all planning docs. Untracked: `CLAUDE.md`, `OBELISK_AGENT_PLAN.md`, this file. Compiles clean. |
| **obelisk-agent** | `../obelisk-agent` | One commit (`e32e40d Initial`). All 6 endpoints + auth middleware written. Compiles clean. **Not production-ready** — see "The agent" below. |
| **obelisk-template** | github.com/nickdill/obelisk-template | External; `obelisk new` downloads it. Does **not** yet include the agent service or nginx `/_obelisk/` proxy block. |

---

## Command status

| Command | Status | Notes |
|---|---|---|
| `obelisk new <name>` | ✅ works | Downloads template tarball, extracts, patches project name |
| `obelisk init` | ✅ works | Writes `obelisk.yml` + `.obelisk/setup.sh` + `run.sh` |
| `obelisk dev` | ✅ works | Runs `.obelisk/dev.sh` (falls back to `run.sh`) |
| `obelisk down` | ✅ works | `docker compose down` |
| `obelisk logs [svc]` | ✅ works | `docker compose logs -f` |
| `obelisk debug` | ✅ works | Prints active `obelisk.yml` / `obelisk.local.yml` |
| `obelisk update` | ⚠️ partial | Prints installer instructions; the `sourceDir` path is never set anywhere in this repo (presumably injected by an external install script via ldflags) |
| `obelisk deploy` | ❌ stub | Prints "coming soon" — the main thing to build |
| `obelisk status` | ❌ stub | Prints "coming soon" — see cleanup candidates |
| `obelisk publish` | ❌ stub | Prints "coming soon" — see cleanup candidates |

**Not yet started** (designed in `CLI_AUTHENTICATION_PLAN.md`, no code): `obelisk identity`, `obelisk server add/list/remove`, `obelisk allow`, `obelisk revoke`, `obelisk list`, and the three internal packages they need (`internal/identity`, `internal/client`, `internal/registry`).

---

## The agent (`../obelisk-agent`)

Built and committed: ED25519 auth middleware, `ping`, `status`, `deploy` (streams `.obelisk/run.sh` output), key management with lockout guard, Dockerfile, ghcr.io release workflow. Stdlib only.

**All 4 critical bugs are fixed and committed on the `develop` branch (`17ab3fd`).** The frozen protocol:
- `PATH` in the canonical string is the agent-side escaped path (`/v1/...`), never the `/_obelisk` prefix
- Fingerprints use `base64url-nopad` (URL-safe, no padding)
- Dockerfile now uses `docker-cli-compose` (v2 plugin)
- Key store reloads from disk on Lookup miss if the file changed (fixes bootstrap ordering)
- Nonce is committed post-verify; 1 MiB body cap; nonce cache capped at 100k
- Deploys serialized (409), 30-min timeout, process-group kill on cancel, audit logged
- Full test suite including golden vectors (`internal/auth/testdata/vectors.json`) — the CLI must consume the same file

The accidental `obelisk.yml` / `.obelisk/` init cruft has been removed from the agent repo.

---

## Obsolete & cleanup candidates

### Docs that are now stale or contradicted

| File | Why |
|---|---|
| `TASK.md` | Describes `obelisk login` + Megalisk account auth — **directly contradicts the decided model.** `CLI_AUTHENTICATION_PLAN.md` locked in decentralized keypairs with explicitly *no* accounts/login. Delete or rewrite. |
| `TASK_MEGALISK_AGENT.md` | The exploration that led to the agent. Agent is now built; superseded. |
| `TASK_SUPPORT_MULTIPLE_DEVS.md` | Superseded by `CLI_AUTHENTICATION_PLAN.md` §6 (the allow/revoke onboarding flow). |
| `PROMPT_HISTORY.md` | Scratch notes. |

### Commands worth reconsidering

- **`obelisk status`** — the planned `obelisk list` (signed status fan-out across all registered servers) covers the remote case. Either rewire `status` to local-only `docker compose ps` or remove it to avoid two overlapping commands.
- **`obelisk publish`** — ECR image push. Still plausible for later, but not on the critical path; the deploy flow works without it. Keep as stub or remove until needed.
- **`obelisk update`** — works only via an external installer that isn't in this repo. Fine to leave, but know it's inert as checked in.

---

## Which doc is authoritative for what

| Doc | Authority |
|---|---|
| `CLI_AUTHENTICATION_PLAN.md` | **Normative** auth spec: wire protocol, key format, threat model, build phases. Both repos implement exactly this. |
| `OBELISK_AGENT_PLAN.md` | Agent spec + decision log (deploy shells out to `.obelisk/run.sh`, alpine image, etc.) |
| `../obelisk-agent/FABLE_ADVICE.md` | Agent work queue — ordered findings from code review |
| `CLAUDE.md` (both repos) | Session context: structure, what's built, what's next |
| `vision.md` | Product vision + `obelisk.yml` schema reference |
| `CLI_SCOPE.md`, `DECIDE.md`, `MARKETING.md` | Historical brainstorming — decisions made there are captured in the docs above |
| `PLAN_SERVER_PROVISIONING.md`, `build_versioning_guide.md` | Future-phase plans, not current work |

---

## Recommended next steps, in order

1. **Fix the agent's 4 critical bugs** + add the shared golden-vector test fixtures (`FABLE_ADVICE.md` §16.1) — this freezes the wire protocol before any CLI client code exists, so the two repos can't drift.
2. **`internal/identity` + `obelisk identity`** — keypair generation, `obk1_` encoding, signing. Pure local, no server needed to test.
3. **`internal/client` + `internal/registry`** — signed HTTP client and `~/.config/obelisk/servers.yml`.
4. **`obelisk server add` + `obelisk list`** — first end-to-end contact with a running agent.
5. **Wire `obelisk deploy`** — replace the stub; the full local → live loop.
6. **Cleanup pass** — delete/rewrite the stale TASK docs, decide on `status`/`publish`, remove the init cruft from the agent repo.
