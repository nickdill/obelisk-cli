package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize current directory as an Obelisk project",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	obeliskDir := filepath.Join(cwd, ".obelisk")
	if err := os.MkdirAll(obeliskDir, 0755); err != nil {
		return fmt.Errorf("could not create .obelisk/: %w", err)
	}

	files := map[string]string{
		"obelisk.yml": obeliskYMLTemplate,
		filepath.Join(".obelisk", "setup.sh"): setupSHTemplate,
		filepath.Join(".obelisk", "run.sh"):   runSHTemplate,
	}

	for path, content := range files {
		full := filepath.Join(cwd, path)
		if _, err := os.Stat(full); err == nil {
			fmt.Printf("  skip  %s (already exists)\n", path)
			continue
		}
		if err := os.WriteFile(full, []byte(content), 0755); err != nil {
			return fmt.Errorf("could not write %s: %w", path, err)
		}
		fmt.Printf("  create %s\n", path)
	}

	fmt.Println("\nProject initialized. Edit obelisk.yml to configure your modules.")
	return nil
}

const obeliskYMLTemplate = `version: "0.1"
name: "my-obelisk"
type: obelisk
modules:
  # example:
  #   image: nginx:latest
  #   port: 80
  #   domain: example.com
`

const setupSHTemplate = `#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$SCRIPT_DIR"

# Install yq if not present
if ! command -v yq > /dev/null 2>&1; then
    echo "[Obelisk] Installing yq..."
    wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
    chmod +x /usr/local/bin/yq
fi

# Load environment
[ -f .env ] && . ./.env

if [ -f obelisk.local.yml ]; then
    CONFIG_FILE=obelisk.local.yml
else
    CONFIG_FILE=obelisk.yml
fi
export CONFIG_FILE

echo "[Obelisk] Setup complete."
`

const runSHTemplate = `#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$SCRIPT_DIR"

if [ -f obelisk.local.yml ]; then
    CONFIG_FILE=obelisk.local.yml
else
    CONFIG_FILE=obelisk.yml
fi
export CONFIG_FILE

echo "[Obelisk] Generating docker-compose override..."
sh .obelisk/scripts/generate-compose.sh

echo "[Obelisk] Generating nginx configs..."
sh .obelisk/scripts/generate-nginx.sh

echo "[Obelisk] Starting services..."
docker compose up -d

docker exec nginx-webserver nginx -s reload 2>/dev/null || true

echo "[Obelisk] Running."
`
