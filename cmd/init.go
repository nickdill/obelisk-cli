package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initModuleMode bool
var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize current directory as an Obelisk project",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVarP(&initModuleMode, "module", "m", false, "Initialize as a module (single app) rather than a server")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing scripts (preserves obelisk.yml)")
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

	var files map[string]string
	var doneMsg string

	if initModuleMode {
		files = map[string]string{
			"obelisk.yml":                    obeliskModuleYMLTemplate,
			filepath.Join(".obelisk", "dev.sh"): moduleDevSHTemplate,
		}
		doneMsg = "Module initialized. Edit obelisk.yml and .obelisk/dev.sh to configure your module."
	} else {
		scriptsDir := filepath.Join(cwd, ".obelisk", "scripts")
		if err := os.MkdirAll(scriptsDir, 0755); err != nil {
			return fmt.Errorf("could not create .obelisk/scripts/: %w", err)
		}
		files = map[string]string{
			"obelisk.yml":        obeliskYMLTemplate,
			"docker-compose.yml": dockerComposeYMLTemplate,
			filepath.Join(".obelisk", "setup.sh"):                       setupSHTemplate,
			filepath.Join(".obelisk", "run.sh"):                         runSHTemplate,
			filepath.Join(".obelisk", "dev.sh"):                         devSHTemplate,
			filepath.Join(".obelisk", "scripts", "generate-compose.sh"): generateComposeSHTemplate,
			filepath.Join(".obelisk", "scripts", "generate-nginx.sh"):   generateNginxSHTemplate,
		}
		doneMsg = "Project initialized. Edit obelisk.yml to configure your modules."
	}

	for path, content := range files {
		full := filepath.Join(cwd, path)
		_, statErr := os.Stat(full)
		alreadyExists := statErr == nil

		if alreadyExists && (path == "obelisk.yml" || !initForce) {
			fmt.Printf("  skip   %s\n", path)
			continue
		}
		if err := os.WriteFile(full, []byte(content), 0755); err != nil {
			return fmt.Errorf("could not write %s: %w", path, err)
		}
		if alreadyExists {
			fmt.Printf("  update %s\n", path)
		} else {
			fmt.Printf("  create %s\n", path)
		}
	}

	if initForce {
		doneMsg = "Scripts updated to latest version. obelisk.yml was not changed."
	}
	fmt.Println("\n" + doneMsg)
	return nil
}

const dockerComposeYMLTemplate = `services:
  nginx-webserver:
    image: nginx:latest
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./.obelisk/nginx:/etc/nginx/conf.d:ro
    networks:
      - obelisk
    restart: unless-stopped

networks:
  obelisk:
    driver: bridge
`

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

const devSHTemplate = `#!/bin/sh
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

echo "[Obelisk] Starting services (dev mode)..."
docker compose up
`

const generateComposeSHTemplate = `#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$SCRIPT_DIR"

CONFIG_FILE="${CONFIG_FILE:-obelisk.yml}"

modules=$(yq e '.modules // {} | keys | .[]' "$CONFIG_FILE")

if [ -z "$modules" ]; then
    printf 'services: {}\n' > docker-compose.override.yml
    echo "[Obelisk] Generated docker-compose.override.yml"
    exit 0
fi

cat > docker-compose.override.yml << 'YAML'
services:
YAML

echo "$modules" | while read -r name; do
    image=$(yq e ".modules.${name}.image" "$CONFIG_FILE")
    port=$(yq e ".modules.${name}.port" "$CONFIG_FILE")
    cat >> docker-compose.override.yml << YAML
  ${name}:
    image: ${image}
    expose:
      - "${port}"
    networks:
      - obelisk
YAML
done

echo "[Obelisk] Generated docker-compose.override.yml"
`

const generateNginxSHTemplate = `#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$SCRIPT_DIR"

CONFIG_FILE="${CONFIG_FILE:-obelisk.yml}"

mkdir -p .obelisk/nginx

yq e '.modules // {} | keys | .[]' "$CONFIG_FILE" | while read -r name; do
    domain=$(yq e ".modules.${name}.domain" "$CONFIG_FILE")
    port=$(yq e ".modules.${name}.port" "$CONFIG_FILE")
    cat > ".obelisk/nginx/${name}.conf" << NGINX
server {
    listen 80;
    server_name ${domain};

    location / {
        proxy_pass http://${name}:${port};
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    }
}
NGINX
done

echo "[Obelisk] Generated nginx configs."
`

const obeliskModuleYMLTemplate = `version: "0.1"
name: "my-module"
type: module
modules:
  my-module:
    image: my-module:latest
    port: 3000
    domain: my-module.localhost
`

const moduleDevSHTemplate = `#!/bin/sh
set -e
# Start this module for local development.
# Replace the command below with your project's dev command, for example:
#   npm run dev
#   go run .
#   python -m flask run --port 3000
#   docker compose up
echo "[Obelisk] No dev command configured. Edit .obelisk/dev.sh to start this module."
exit 1
`
