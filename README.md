# Obelisk

**Deploy multiple projects to a single server. No Kubernetes. Just `obelisk.yml`.**

Obelisk is a deployment framework for indie developers and small teams who want to self-host multiple apps without the complexity of a full orchestration platform. Define your services in one config file — Obelisk handles nginx routing, SSL certificates, and Docker Compose wiring automatically.

> **Early development.** The config format and CLI are subject to change.

---

## Getting started

```bash
# Install
curl -fsSL https://obelisk.dev/install.sh | bash

# Create a new project
obelisk new my-project
cd my-project

# Add your services to obelisk.yml, then:
obelisk dev        # run locally
obelisk deploy     # deploy to your server
```

Full documentation coming soon.

That's it. Nginx configs, Docker Compose, and Let's Encrypt are all handled for you.

---

## The problem
Running multiple apps on one server means:

- Writing and maintaining nginx configs for each service
- Wiring Docker Compose for each app separately
- Setting up Let's Encrypt and keeping certificates renewed
- Keeping all of it in sync every time something changes

Obelisk generates all of it from one config file. Add a service, run one command, done.

---

## Quick look

```yaml
# obelisk.yml
version: "0.1"
name: my-project
type: obelisk

modules:
  api:
    git_source: https://github.com/myorg/api
    port: 3000
    domain: api.example.com

  web:
    image: myorg/web:latest
    port: 8080
    domain: example.com

  docs:
    git_source: https://github.com/myorg/docs
    port: 4000
    domain: docs.example.com
    type: static
```

```bash
obelisk dev      # run everything locally with self-signed SSL
obelisk deploy   # ship to production with real certificates
```

For local development, create an `obelisk.local.yml` with the same format but using relative local paths for `git_source`. Obelisk automatically uses the local config when it's present.

---

## Features

- **Domain-based routing** — each module gets its own domain, routed through a shared nginx reverse proxy
- **Automatic SSL** — Let's Encrypt certificates provisioned and auto-renewed via Certbot
- **Local dev mode** — self-signed certs, local paths, certbot disabled — same workflow everywhere
- **Bring your own source** — point at a Docker image (ECR, Docker Hub, any registry) or a git repo
- **Static site support** — SPAs and static files served directly by nginx; no extra container in production
- **Webserver-level observability** — nginx access logs cover every service in one place, no per-app instrumentation needed
- **DDoS protection** — rate limiting and request filtering at the nginx layer, before traffic reaches your apps

---

## How it works

1. **Declare your services** in `obelisk.yml` — name, source, port, domain
2. **`obelisk deploy`** generates a `docker-compose.override.yml` and per-service nginx configs from templates
3. **Everything starts** behind a single nginx reverse proxy on the shared Docker network
4. **SSL is handled automatically** — Let's Encrypt in production, self-signed locally

No manual config files. No restarting nginx by hand. No keeping compose files in sync with your service list.

---

## Roadmap

### Now — v0.1
- [x] `obelisk.yml` config format
- [x] Automatic nginx config generation
- [x] Automatic Docker Compose generation
- [x] Let's Encrypt SSL + Certbot
- [x] Local dev mode (`obelisk.local.yml`)
- [x] Static site module support
- [x] AWS CodeDeploy integration

### Coming soon
- [ ] `obelisk deploy` — one-command SSH deploy to any server
- [ ] `obelisk publish` — build and push module images to a registry
- [ ] `obelisk status` — health check and status summary for all running services
- [ ] `obelisk logs [module]` — stream logs for a specific service

### Future
- [ ] Webserver-level analytics dashboard — traffic and error rates across all services
- [ ] `obelisk scale` — horizontal scaling across multiple servers
- [ ] DNS management — automatic Route 53 record configuration
- [ ] EC2 provisioning — spin up and configure servers from the CLI
- [ ] CloudFront / S3 for static modules — CDN delivery without changing your config
- [ ] Registry integration — `obelisk publish` push to ECR, Docker Hub, or any registry
