# Obelisk

Obelisk is a deployment framework for running multiple projects on a single server. You declare your services in one YAML file, and Obelisk handles the rest: nginx routing, SSL certificates, and Docker Compose orchestration — no Kubernetes, no DevOps team required.

---

## The Problem

When you want to host multiple apps on a single server, you have to manually:
- Write and maintain nginx configs for each service
- Wire up Docker Compose for each app
- Configure Let's Encrypt SSL and keep it renewed
- Keep all of this in sync as services are added or changed

Obelisk automates all of it. You define what you want to run; Obelisk generates the configuration.

---

## Core Principles

- **No Kubernetes.** Docker Compose + nginx is the entire runtime. Simple enough to understand, simple enough to debug.
- **Single-server, self-hosted.** One server. You own the machine.
- **Config-driven.** One `obelisk.yml` declares every service. No manual config editing.
- **Bring your own source.** Modules can be Docker images (ECR or any registry) or Git repositories built on the server.

---

## How It Works

An Obelisk project is a directory containing an `obelisk.yml` and a `.obelisk/` directory with orchestration scripts. When you start the project:

1. **`obelisk.yml` is read.** Each entry under `modules` defines a service.
2. **`generate-compose.sh` runs.** It produces `docker-compose.override.yml`, adding one container per module with dynamic port assignment starting at 4000.
3. **`generate-nginx.sh` runs.** It produces a per-module nginx config under `nginx/data/nginx/`, using templates that handle HTTP→HTTPS redirects, SSL termination, and reverse proxying.
4. **`docker compose up -d` starts everything.** This includes nginx, certbot, and all module containers on the shared `obelisk` Docker network.
5. **Nginx reloads.** The new configs take effect and traffic is routed by domain to each container.
6. **Certbot auto-renews** Let's Encrypt certificates every 12 hours.

Nginx operates as the single entry point. All traffic on ports 80/443 flows through it. Each module gets a dedicated nginx config that proxies requests to its container by name over the internal Docker network.

---

## Key Concepts

**Project** — A deployment unit. One directory, one `obelisk.yml`, one set of orchestration scripts. A project runs on one server.

**Module** — An individual service within a project. A module has a source (image or git repo), a container port, and a domain. Each module becomes one running Docker container.

**Static module** — A module that serves static files (e.g. a compiled React SPA). In local development, its build output is volume-mounted directly into nginx. In production, it has no container — nginx serves the files directly.

**obelisk.yml** — The canonical config file for a project. Defines all modules and their routing.

**obelisk.local.yml** — Local development override. When this file is present, Obelisk uses it instead of `obelisk.yml`. Typically uses relative local paths instead of git URLs, and disables certbot.

**.obelisk/** — The orchestration directory embedded in every project. Contains the setup and run scripts, config generators, and nginx templates. These are checked into the project repo.

---

## obelisk.yml Schema

```yaml
version: "0.1"          # Config format version
name: "my-project"      # Human-readable project name
type: obelisk           # Always "obelisk" for a project

modules:
  api:                  # Module name (used as container name and in nginx config)
    image: 123456789.dkr.ecr.us-east-2.amazonaws.com/my-api:latest
    port: 3000          # Port the container listens on
    domain: api.example.com
    env:
      DATABASE_URL: postgres://...
      NODE_ENV: production

  web:
    git_source: https://github.com/myorg/web-app
    port: 8080
    domain: example.com

  docs:
    git_source: ../docs-site   # Local path (obelisk.local.yml only)
    port: 4000
    domain: docs.localhost
    type: static               # Static file module — no container in production
```

### Module fields

| Field | Required | Description |
|---|---|---|
| `image` | if no `git_source` | Docker image to pull and run |
| `git_source` | if no `image` | Git URL to clone, or relative/absolute local path |
| `port` | yes | Port the container exposes |
| `domain` | yes | Domain for nginx routing |
| `type` | no | Set to `static` for static file modules |
| `env` | no | Environment variables passed to the container |

### Module source resolution

- **Git URL** (`https://...`) — cloned into `./modules/<name>/` during setup
- **Relative path** (`./...`, `../...`) — used as-is; ideal for local dev
- **Absolute path** (`/...`) — used as-is
- **No `git_source`** — expects `image` to be set; image is pulled rather than built

---

## Dev vs. Production

| | Local development | Production |
|---|---|---|
| Config file | `obelisk.local.yml` | `obelisk.yml` |
| Module source | Relative local paths | Git repos or Docker images |
| SSL | Self-signed certs (`nginx/data/certs/`) | Let's Encrypt (Certbot) |
| Certbot | Disabled (no-op container) | Enabled, renews every 12h |
| Static modules | Volume-mounted into nginx | No container; nginx serves files directly |
| Nginx reload | On each `run` | On each `run` + every 6h automatically |

Local mode is activated automatically when `obelisk.local.yml` exists. No flags needed.

---

## Project Directory Structure

```
obelisk.yml                         # Production config
obelisk.local.yml                   # Dev overrides (gitignored)
.env                                # Secrets (gitignored)
docker-compose.yml                  # Base services: nginx + certbot
docker-compose.override.yml         # Generated on each run — do not edit manually

.obelisk/
  setup.sh                          # Run once: clone repos, authenticate to ECR, pull images
  run.sh                            # Run each time: generate configs, start services
  dev.sh                            # Dev variant of run.sh
  stop.sh                           # docker compose down
  scripts/
    generate-compose.sh             # Writes docker-compose.override.yml
    generate-nginx.sh               # Writes nginx/data/nginx/<module>.conf

nginx/
  conf/default.conf                 # Default nginx server (fallback / landing page)
  templates/
    module.conf.tmpl                # Production proxy template (HTTPS + Let's Encrypt)
    module.local.conf.tmpl          # Dev proxy template (HTTP + self-signed)
    module.local.static.conf.tmpl   # Dev static file template
  data/
    nginx/                          # Generated per-module nginx configs (do not edit)
    certbot/                        # Let's Encrypt cert storage
    certs/                          # Self-signed dev certs

modules/                            # Cloned git repos land here
```

Files marked "do not edit" are regenerated on every run.

---

## Getting Started

The recommended way to scaffold an Obelisk project is with the [Megalisk CLI](https://github.com/nickdill/megalisk-cli):

```bash
mega new my-project
cd my-project
```

This downloads the Obelisk template, giving you the full `.obelisk/` structure, `docker-compose.yml`, nginx templates, and a starter `obelisk.yml`.

Then:
1. Edit `obelisk.yml` to declare your modules
2. Create `obelisk.local.yml` with local paths for development
3. Run `mega run` to generate configs and start services

For local development without the CLI:
```bash
./.obelisk/setup.sh    # first time only
./.obelisk/run.sh
```

---

## Production Deployment

### Current: AWS CodeDeploy

The Obelisk template includes an `appspec.yml` for AWS CodeDeploy. When a deployment is triggered, CodeDeploy runs:

1. `BeforeInstall` → `.obelisk/setup.sh` — clones repos, authenticates to ECR, pulls images
2. `ApplicationStart` → `.obelisk/run.sh` — generates configs, starts Docker Compose, reloads nginx

The `.env` file on the server provides AWS credentials and any secrets not committed to the repo.

### Future: `mega deploy`

The `mega deploy` command (currently a stub in the Megalisk CLI) will automate the full production deployment flow: SSH to the server, pull the latest config, run setup and run scripts. The goal is a single command to go from local to live.

---

## Additional Capabilities

Because every Obelisk deployment runs a centralized nginx instance, there are capabilities that are difficult or expensive to get elsewhere:

- **Webserver-level metrics and tracking** — nginx access logs cover all services in one place, enabling cross-service observability without instrumenting each app individually
- **DDoS protection** — rate limiting, connection throttling, and request filtering can be applied at the nginx layer before traffic reaches any app
- **Centralized logging** — nginx logs can be shipped remotely for all hosted services with a single configuration change

---

## Future Direction

Obelisk is designed to optionally layer AWS services on top of the core Docker + nginx runtime:

- **DNS management** — point a domain at Obelisk and it configures Route 53 records automatically
- **EC2 provisioning** — spin up and configure new server instances from the CLI
- **ECR integration** — `mega publish` builds and pushes module images to ECR; `mega deploy` pulls them to production
- **CloudFront / S3** — serve static modules via CDN rather than the origin server

These are additive. The core framework (Docker Compose + nginx + `obelisk.yml`) remains the foundation.

---

## Relationship to Megalisk

[Megalisk](https://github.com/nickdill/megalisk-cli) is a separate project that uses Obelisk as its deployment layer. Megalisk provides:

- A CLI (`mega`) for creating and managing Obelisk projects
- A Rails dashboard for tracking servers and deployments

Megalisk is not part of Obelisk. Obelisk projects can be run without Megalisk — the `.obelisk/` scripts are self-contained. Megalisk is the recommended companion tooling.
