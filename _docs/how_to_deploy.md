# How to Deploy
Since the CLI is not a deployed app we just need to compile Go builds and push them to github to be pulled for installs.


### Git Tag Latest
./tag-release.sh

Push to origin (prints the command)

Github actions will create the release

TBD order of operations taggings vs build targets (below) which is versioned first?


---

Manual steps:

Unknowns, release candidates, beta versions, staging builds, is that just not pushing the `gh release` call until after testing in staging/pre env? Must be


## Build artifact naming

The asset name pattern (from both install.sh and cmd/update.go) is:

obelisk-{OS}-{ARCH}

Examples:
- obelisk-darwin-arm64
- obelisk-darwin-amd64
- obelisk-linux-amd64
- obelisk-linux-arm64

## Where they must be uploaded

GitHub Releases for the repo nickdill/obelisk-cli. The install script and self-updater both resolve assets at:
https://github.com/nickdill/obelisk-cli/releases/download/{tag}/{asset}

## How to build and publish a release

Build all targets:
GOOS=darwin  GOARCH=arm64 go build -ldflags "-X cmd.version=v0.1.0" -o obelisk-darwin-arm64 .
GOOS=darwin  GOARCH=amd64 go build -ldflags "-X cmd.version=v0.1.0" -o obelisk-darwin-amd64 .
GOOS=linux   GOARCH=amd64 go build -ldflags "-X cmd.version=v0.1.0" -o obelisk-linux-amd64  .
GOOS=linux   GOARCH=arm64 go build -ldflags "-X cmd.version=v0.1.0" -o obelisk-linux-arm64  .

### Tag and create a GitHub release, attaching the binaries:
git tag v0.1.0 && git push origin v0.1.0
gh release create v0.1.0 obelisk-darwin-arm64 obelisk-darwin-amd64 obelisk-linux-amd64 obelisk-linux-arm64

The install.sh hits the GitHub API for releases/latest to find the tag, then downloads the matching asset by OS/arch.
obelisk update does the same via the GitHub API.
