# Build Source Decision: install.sh stays public-only

## Decision

`install.sh` targets a public GitHub release. No `GITHUB_TOKEN` support will be added.

## Reasoning

**Supporting public + private adds real complexity for a temporary problem.**

The private-repo state ends at launch. Supporting both code paths means:
- Switching the download mechanism from a direct release URL to the GitHub assets API (required because GitHub redirects to S3 on direct downloads, stripping auth headers — the naive `curl -H "Authorization: ..." $URL` approach silently downloads an HTML error page and installs a broken binary)
- Testing two distinct code paths in the installer
- Documenting `GITHUB_TOKEN` in the README for a flow that disappears the moment the repo is public

**The right tool for private access is `local-install.sh`.**

Anyone who needs obelisk before the public release is a developer on this project. They have the repo cloned and can run `./local-install.sh`. There's no scenario where a non-developer needs to install from a private repo via a curl-pipe-bash.

**The "Not Found" error is acceptable for now.**

If someone does stumble onto `install.sh` while the repo is private, the script exits with a clear message. No silent failures, no corrupted binary.

## When to revisit

If obelisk ever needs a private distribution channel (enterprise tier, invite-only beta), add `GITHUB_TOKEN` support at that point — don't build it speculatively now.
