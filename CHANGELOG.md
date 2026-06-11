# Changelog

All notable changes to obelisk-cli are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **Server connectivity and auth** — the CLI can now communicate with deployed Obelisk servers over a signed HTTPS protocol. No SSH required after initial bootstrap.
- `obelisk identity` — generate and display your local ED25519 keypair (`~/.config/obelisk/id_ed25519`). Prints `obk1_...` public key and `SHA256:...` fingerprint. Generates on first run; `--force` to rotate.
- `obelisk server add <name> <url>` — register a server and verify connectivity with a signed handshake. Prints agent version on success; shows your public key with onboarding instructions on a 403.
- `obelisk server list` — table of registered servers with name, URL, and last-seen time.
- `obelisk server remove <name>` — remove a server from the local registry.
- `obelisk allow <pubkey> [--name <label>] [--server <name>]` — authorize a teammate's public key on a server (`POST /v1/keys`).
- `obelisk revoke <fingerprint> [--server <name>]` — revoke a key from a server (`DELETE /v1/keys/{fingerprint}`).
- `obelisk list` — fan out `GET /v1/status` to all registered servers in parallel and render a combined module-state table.
- `obelisk deploy` — fully implemented (replaces the previous stub). Reads the module name from `obelisk.yml`, resolves the target server, streams deploy output live, and exits with the agent's exit code. Flags: `--server <name>`.
- `internal/identity` — ED25519 keypair management, `obk1_` encoding, `SHA256:` fingerprinting, and request signing. Wire format verified against the shared golden test vectors in `obelisk-agent`.
- `internal/client` — signed HTTP client that attaches `X-Obelisk-Key`, `X-Obelisk-Timestamp`, `X-Obelisk-Nonce`, and `X-Obelisk-Signature` headers. Enforces HTTPS for non-localhost targets. Surfaces human-readable errors for unknown-key and clock-skew rejections.
- `internal/registry` — local server registry at `~/.config/obelisk/servers.yml` with `Add`, `Remove`, `List`, and `Resolve` (auto-selects when only one server is registered).
- `obelisk status` — shows project name, init state, and module list (ports + domains) for server projects; shows module name and port for module repos
- `obelisk uninstall` — removes all Obelisk-managed files; detects project type and removes the appropriate set (server: `docker-compose.yml`, `obelisk.yml`, `.obelisk/`; module: `obelisk.yml`, `.obelisk/`)
- `obelisk init` creates `.env` with `OBELISK_HTTP_PORT`/`OBELISK_HTTPS_PORT` overrides so multiple local projects can run on different ports simultaneously
- `obelisk init` auto-adds `.env` to `.gitignore`, creating the file if absent
- `obelisk init` accepts `install` and `i` as aliases
- `generate-compose.sh` supports modules with `git_source` — builds from a local path or git URL via Docker Compose `build.context`
- `obelisk dev` warns when the installed `generate-compose.sh` predates `git_source` support and prompts `obelisk init --force` to update it
- `internal/config`: `IsModule()`, `ModuleConfig` struct, and `LoadModule()`

### Changed

- Config file naming unified: both server projects and module repos now use `obelisk.yml`, distinguished by a `type:` field (`type: server` vs `type: module`). Previously module repos used `obelisk.module.yml`.
- `docker-compose.yml` template uses `${OBELISK_HTTP_PORT:-80}` / `${OBELISK_HTTPS_PORT:-443}` instead of hardcoded ports
- `obelisk status` module view uses `ModuleConfig` and renders `Module: <name>` / `Port: <n>` instead of the server-style module table
- `obelisk uninstall` always removes `obelisk.yml`; the `--purge` flag that previously guarded it has been removed

### Fixed

- `obelisk dev` no longer errors with a config-not-found message when run in a project without `obelisk.yml`; the staleness check is skipped when no config is present
