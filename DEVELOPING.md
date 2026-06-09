# Developer Guide

## Working locally

The fastest way to test changes is to build and run the binary in place:

```bash
go build -o obelisk .
./obelisk --help
```

Or use `go run` to skip the build step entirely:

```bash
go run . --help
go run . new my-project
go run . debug
```

To install the current working version into your PATH so you can use `obelisk` from anywhere:

```bash
go build -o ~/.local/bin/obelisk .
```

After that, `obelisk` in any terminal points to what you just built.

## Iterating on changes

When you make a change and want to test it:

```bash
go build -o ~/.local/bin/obelisk . && obelisk --help
```

That rebuilds and immediately makes the new version available. No need to uninstall first.

## Removing the local install

```bash
rm ~/.local/bin/obelisk
```

## Shipping a release

Releases are automated via GitHub Actions (`.github/workflows/release.yml`). Pushing a version tag triggers a build of four binaries and publishes them as a GitHub Release.

**Steps:**

1. Commit everything and push to `main`
2. Tag the release:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```
3. GitHub Actions builds `obelisk-darwin-amd64`, `obelisk-darwin-arm64`, `obelisk-linux-amd64`, `obelisk-linux-arm64` and attaches them to the release automatically.

Check build progress at: `https://github.com/nickdill/obelisk/actions`

Once the workflow completes, the `install.sh` script will work for anyone:

```bash
curl -fsSL https://raw.githubusercontent.com/nickdill/obelisk/main/install.sh | bash
```

## Tagging conventions

Use [semantic versioning](https://semver.org): `vMAJOR.MINOR.PATCH`

- `v0.x.0` — pre-1.0, breaking changes are expected
- `v1.0.0` — stable public API
- Patch bumps (`v0.1.1`) for bug fixes, minor bumps (`v0.2.0`) for new commands or behavior changes

## Fixing a bad release

If you need to redo a tag (e.g. forgot to commit something):

```bash
git tag -d v0.1.0                  # delete local tag
git push origin --delete v0.1.0    # delete remote tag
# make your fix, then re-tag
git tag v0.1.0
git push origin v0.1.0
```

Note: deleting a tag that already has a GitHub Release won't delete the release itself — you may also need to delete the release draft in the GitHub UI before re-pushing.

## Installing a specific version

The installer respects the `OBELISK_VERSION` env var:

```bash
OBELISK_VERSION=v0.1.0 bash install.sh
```

Useful for rolling back if a new release has a problem.
