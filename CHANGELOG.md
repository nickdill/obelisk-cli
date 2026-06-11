# Changelog

All notable changes to obelisk-cli are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

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
