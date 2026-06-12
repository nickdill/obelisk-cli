# Obelisk Authentication Quickstart

This guide explains how to get a new developer authenticated against a deployed Obelisk server — from zero to `obelisk deploy` working.

---

## How it works

Obelisk uses **ED25519 keypairs** to authenticate every request. There are no accounts, no passwords, and no central login service. Think of it as the SSH `authorized_keys` model applied to HTTP.

Each developer has a keypair stored locally at `~/.config/obelisk/`. Every CLI request is cryptographically signed with their private key. The server holds a list of trusted public keys (`allowed_keys.json`) and verifies each signature before doing anything.

### Architecture overview

```
  Your laptop                          Obelisk server
 ┌─────────────────────┐              ┌────────────────────────────────────────┐
 │                     │              │                                        │
 │  obelisk CLI        │              │  nginx (port 443, Let's Encrypt TLS)  │
 │                     │──signed──────►    └── /_obelisk/* ──► obelisk-agent  │
 │  ~/.config/obelisk/ │   HTTPS      │                              │        │
 │    id_ed25519       │              │                    allowed_keys.json  │
 │    id_ed25519.pub   │              │                    docker socket      │
 │    servers.yml      │              │                                        │
 └─────────────────────┘              └────────────────────────────────────────┘
```

- **nginx** terminates TLS. The agent is never exposed directly to the internet — it only listens on the internal Docker network.
- **obelisk-agent** verifies the signature, checks that the key is on the allowed list, and executes a fixed set of operations (deploy, status, key management). It cannot execute arbitrary commands.
- **Your private key never leaves your laptop.** Intercepting a request reveals nothing reusable.

---

## The handshake, step by step

```
  CLI                                         obelisk-agent
   │                                               │
   │  1. Generate nonce + timestamp                │
   │  2. Sign: v1 / METHOD / PATH /                │
   │           timestamp / nonce /                 │
   │           sha256(body)                        │
   │                                               │
   │──── HTTPS POST /_obelisk/v1/deploy ──────────►│
   │     X-Obelisk-Key:       obk1_...             │
   │     X-Obelisk-Timestamp: 1749600000           │
   │     X-Obelisk-Nonce:     a3f8...              │
   │     X-Obelisk-Signature: base64...            │
   │                                               │
   │                        3. Verify timestamp    │
   │                           (must be within     │
   │                            ±60 seconds)       │
   │                                               │
   │                        4. Look up key in      │
   │                           allowed_keys.json   │
   │                           (403 if unknown)    │
   │                                               │
   │                        5. Verify signature    │
   │                           against canonical   │
   │                           string              │
   │                                               │
   │                        6. Check nonce hasn't  │
   │                           been seen before    │
   │                           (replay protection) │
   │                                               │
   │◄──── 200 OK (streaming output) ───────────────│
```

Requests are replay-proof: the timestamp window kills anything older than 60 seconds, and the per-key nonce cache catches anything within that window.

---

## Scenario A: You're the first developer (setting up a new server)

**Step 1 — Generate your keypair and copy your public key**

```bash
obelisk identity
```

Output:
```
Public key:  obk1_MCowBQYDK2VwAyEA...
Fingerprint: SHA256:abc123...

Send your public key to a server admin to get access.
```

**Step 2 — Seed your key before provisioning**

In your Obelisk server project's `.env`, set:

```
OBELISK_AUTHORIZED_KEY="obk1_MCowBQYDK2VwAyEA... Your Name"
```

The agent's `setup.sh` seeds this into `data/agent/allowed_keys.json` on first boot — and only on first boot, so re-running setup never clobbers a live key list.

**Step 3 — Register the server locally**

```bash
obelisk server add prod https://obelisk.myteam.com
```

This performs a signed `GET /v1/ping` handshake. On success the server is saved to `~/.config/obelisk/servers.yml`.

**Step 4 — Deploy**

```bash
cd my-module
obelisk deploy
```

---

## Scenario B: You're a new teammate (Alice already has access)

```
  Bob (new developer)                    Alice (existing admin)
       │                                         │
       │  obelisk identity                       │
       │  → copies obk1_BOB...                   │
       │                                         │
       │──── sends obk1_BOB... over Slack ───────►│
       │                                         │
       │                   obelisk allow obk1_BOB \
       │                     --name "Bob"         │
       │                     --server prod        │
       │                                         │
       │  obelisk server add prod                 │
       │    https://obelisk.myteam.com            │
       │  → handshake succeeds ✓                  │
       │                                         │
       │  obelisk deploy                          │
       │  → works ✓                               │
```

**Bob (you) runs:**

```bash
# 1. Get your public key
obelisk identity
# Copy the "Public key: obk1_..." line

# 2. Send it to Alice (Slack, email, whatever)

# 3. Once Alice confirms she's added you, register the server
obelisk server add prod https://obelisk.myteam.com

# 4. Deploy
obelisk deploy
```

**Alice runs:**

```bash
obelisk allow obk1_BOB_FULL_KEY_HERE --name "Bob" --server prod
```

---

## Where the key lives

| File | Location | Purpose |
|---|---|---|
| Private key | `~/.config/obelisk/id_ed25519` | Signs every request. Mode `0600` — never shared. |
| Public key | `~/.config/obelisk/id_ed25519.pub` | Can be shared freely. This is the `obk1_...` string. |
| Server registry | `~/.config/obelisk/servers.yml` | Maps server names to URLs. Local only, not in git. |

The server's copy of trusted keys lives at `data/agent/allowed_keys.json` inside the Obelisk project on the server.

---

## Common error situations

**`403 unknown_key` when adding a server**

Your key isn't on the server's allowed list yet. The CLI will print your public key automatically. Send it to someone who already has access and ask them to run `obelisk allow`.

**`401 stale_timestamp`**

Your system clock is significantly out of sync with the server (more than 60 seconds). Sync your clock (`sudo sntp -sS time.apple.com` on macOS) and retry.

**`401 replay`**

The same signed request was submitted twice. This is usually a network retry issue. Just run the command again — a fresh nonce will be generated.

---

## Revoking access

```bash
# List keys on a server to find the fingerprint
obelisk allow --list --server prod   # (coming soon — for now check server logs)

# Revoke by fingerprint
obelisk revoke SHA256:abc123... --server prod
```

The server refuses to delete the last remaining key, so you can't accidentally lock yourself out.

---

## Key rotation

If your private key is compromised:

1. From **another authorized device**, revoke your old fingerprint:
   ```bash
   obelisk revoke SHA256:OLD_FINGERPRINT --server prod
   ```
2. On your new/clean device, generate a new keypair:
   ```bash
   obelisk identity --force
   ```
3. Send your new public key to an admin to re-authorize.

If you have no second device, the recovery path is SSH access to the server to manually edit `data/agent/allowed_keys.json`.
