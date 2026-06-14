```
       /\        ___   ____   _____  _      ___   ____  _  __
      /  \      / _ \ | __ ) | ____|| |    |_ _| / ___|| |/ /
     /    \    | | | ||  _ \ |  _|  | |     | |  \___ \| ' /
    /      \   | |_| || |_) || |___ | |___  | |   ___) || . \
   /________\   \___/ |____/ |_____||_____||___| |____/ |_|\_\
```

**Deploy multiple projects to a single server. No Kubernetes. Just `obelisk.yml`.**

Obelisk is a deployment framework for indie developers and small teams who want to self-host multiple apps without the complexity of a full orchestration platform. Define your services in one config file — Obelisk handles nginx routing, SSL certificates, and Docker Compose wiring automatically.

> **Early development.** The config format and CLI are subject to change.

---

## Getting started

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/nickdill/obelisk-cli/main/install.sh | bash

# Create a new project
obelisk new my-project
cd my-project

# Add your services to obelisk.yml, then:
obelisk dev        # run locally
obelisk deploy     # deploy to your server
```

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
type: server

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
- **Signed deploys** — every CLI request is signed with your local ED25519 key; no SSH, no shared secrets
- **Team access control** — add and revoke teammates via `obelisk allow` / `obelisk revoke`
- **Webserver-level observability** — nginx access logs cover every service in one place, no per-app instrumentation needed
- **DDoS protection** — rate limiting and request filtering at the nginx layer, before traffic reaches your apps

---

## Deploying to a server

Obelisk servers expose a signed HTTPS API (via `obelisk-agent`) — no SSH required after initial setup.

### First-time setup

```bash
# 1. Generate your identity key (run once per machine)
obelisk identity
#   Public key:  obk1_...
#   Fingerprint: SHA256:...

# 2. Paste the public key into your server's .env before provisioning:
#   OBELISK_AUTHORIZED_KEY="obk1_... Your Name"

# 3. After the server is running, register it locally
obelisk server add prod https://obelisk.myteam.com

# 4. Deploy
obelisk deploy
```

### Adding a teammate

```bash
# Teammate runs on their machine:
obelisk identity   # → copies their obk1_... key

# You run:
obelisk allow obk1_THEIRKEY --name "Alice" --server prod

# Teammate registers the server and can deploy:
obelisk server add prod https://obelisk.myteam.com
obelisk deploy
```

### Server commands

| Command | Description |
|---|---|
| `obelisk identity` | Show your public key and fingerprint |
| `obelisk server add <name> <url>` | Register a server and verify connectivity |
| `obelisk server list` | List registered servers |
| `obelisk server remove <name>` | Unregister a server |
| `obelisk allow <pubkey>` | Authorize a key on a server |
| `obelisk revoke <fingerprint>` | Revoke a key from a server |
| `obelisk list` | Show module status across all servers |
| `obelisk deploy` | Deploy the current module |

---

## How it works

1. **Declare your services** in `obelisk.yml` — name, source, port, domain
2. **`obelisk deploy`** generates a `docker-compose.override.yml` and per-service nginx configs from templates
3. **Everything starts** behind a single nginx reverse proxy on the shared Docker network
4. **SSL is handled automatically** — Let's Encrypt in production, self-signed locally

No manual config files. No restarting nginx by hand. No keeping compose files in sync with your service list.

---

## Roadmap

### v0.1 — local development
- [x] `obelisk.yml` config format
- [x] Automatic nginx config generation
- [x] Automatic Docker Compose generation
- [x] Let's Encrypt SSL + Certbot
- [x] Local dev mode (`obelisk.local.yml`)
- [x] Static site module support

### v0.2 — server connectivity
- [x] ED25519 identity and signed requests (`obelisk identity`)
- [x] Server registry (`obelisk server add/list/remove`)
- [x] `obelisk deploy` — one-command deploy over signed HTTPS
- [x] `obelisk list` — status across all registered servers
- [x] Team key management (`obelisk allow` / `obelisk revoke`)
- [x] `obelisk scale` — set Swarm replica count for a module
- [x] `obelisk update [version]` — self-updating binary from GitHub Releases
- [x] `obelisk build` / `run` / `stop` — module build and production service lifecycle

### Coming soon
- [ ] `obelisk publish` — build and push module images to a registry
- [ ] Key rotation and passphrase-encrypted private keys
- [ ] Per-key permissions and scoping

### Future
- [ ] Webserver-level analytics dashboard — traffic and error rates across all services
- [ ] DNS management — automatic Route 53 record configuration
- [ ] EC2 provisioning — spin up and configure servers from the CLI
- [ ] CloudFront / S3 for static modules — CDN delivery without changing your config
