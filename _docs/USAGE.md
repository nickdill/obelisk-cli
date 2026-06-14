# Obelisk CLI — Usage Reference

## Local development

```bash
obelisk new <name>        # scaffold a new project from the template (downloads obelisk-template)
obelisk init              # initialize current dir as a server project (downloads obelisk-template)
obelisk init --force      # re-download and update scripts; preserves obelisk.yml and .env
obelisk init --module     # initialize as a module repo (no network — writes two local files)
obelisk dev               # start all services (runs .obelisk/dev.sh)
obelisk dev --build       # build images via docker compose before starting
obelisk build             # compile the current module (.obelisk/build.sh)
obelisk run               # start services in production mode (.obelisk/run.sh — Docker Swarm)
obelisk stop              # stop all running services (.obelisk/stop.sh)
obelisk down              # tear down the Docker Swarm stack (docker stack rm)
obelisk logs <module>     # tail logs for a specific module (docker service logs)
obelisk status            # show project type, init state, and running container states
obelisk debug             # print the active obelisk.yml / obelisk.local.yml to stdout
obelisk uninstall         # remove all Obelisk-managed files from the current directory
```

---

## Identity

Every developer needs a local ED25519 keypair to authenticate against Obelisk servers.

```bash
obelisk identity          # show public key + fingerprint; generates keypair on first run
obelisk identity --force  # rotate keypair (destructive — revoke the old key first)
```

Output:
```
Public key:  obk1_MCowBQYDK2VwAyEA...
Fingerprint: SHA256:abc123...

Send your public key to a server admin to get access.
```

Keys are stored at `~/.config/obelisk/id_ed25519` (private, mode 0600).

---

## Server management

```bash
obelisk server add <name> <url>    # register a server and verify connectivity
obelisk server list                # list registered servers (name, URL, last seen)
obelisk server remove <name>       # unregister a server
```

`server add` performs a signed handshake. If the server doesn't know your key yet,
it will print your public key and tell you what to do next.

---

## Team access

```bash
# Authorize a teammate's key
obelisk allow obk1_THEIRKEY --name "Alice" --server prod

# Revoke a key by fingerprint
obelisk revoke SHA256:abc123... --server prod
```

The `--server` flag is optional when only one server is registered.

---

## Deploying

Run from a module repo (a directory with `type: module` in `obelisk.yml`):

```bash
obelisk deploy                  # deploy to the registered server
obelisk deploy --server staging # deploy to a specific server
```

Deploy output is streamed live. The command exits with the remote script's exit code.

---

## Viewing status

```bash
obelisk list                        # fan out GET /v1/status to all registered servers
obelisk scale <module> <replicas>   # set replica count for a module on a server
obelisk scale <module> <replicas> --server staging
```

`obelisk list` output:
```
SERVER    URL                            MODULE   STATE    HEALTH
prod      https://obelisk.myteam.com     api      running  healthy
prod      https://obelisk.myteam.com     web      running
staging   https://staging.example.com    api      exited
```

---

## Updating the CLI

```bash
obelisk update            # update to the latest release
obelisk update 1.2.3      # update (or downgrade) to a specific version
```

Downloads the correct binary for your OS and architecture from GitHub Releases and atomically replaces the running binary. If you are already on the target version, the command exits early.

---

## First-time server setup

1. Run `obelisk identity` and copy your `obk1_...` public key.
2. Add it to your server's `.env` before provisioning:
   ```
   OBELISK_AUTHORIZED_KEY="obk1_... Your Name"
   ```
3. The agent seeds this key into `allowed_keys.json` on first boot.
4. Register the server locally:
   ```bash
   obelisk server add prod https://obelisk.myteam.com
   ```
5. Deploy:
   ```bash
   obelisk deploy
   ```

---

## Onboarding a teammate

```bash
# Teammate (Bob) runs:
obelisk identity   # → "Public key: obk1_BOB..."

# You (Alice) run:
obelisk allow obk1_BOB --name "Bob" --server prod

# Bob registers the server and deploys:
obelisk server add prod https://obelisk.myteam.com
obelisk deploy
```
