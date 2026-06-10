#!/bin/sh
set -e

cd "$(dirname "$0")"

mkdir -p ~/.local/bin
go build -o ~/.local/bin/obelisk .

echo "Installed $(~/.local/bin/obelisk --version) to ~/.local/bin/obelisk"
