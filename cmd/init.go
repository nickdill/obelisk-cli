package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initModuleMode bool
var initForce bool

var initCmd = &cobra.Command{
	Use:     "init",
	Aliases: []string{"install", "i"},
	Short:   "Initialize current directory as an Obelisk project",
	RunE:    runInit,
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

	var doneMsg string

	if initModuleMode {
		files := map[string]string{
			"obelisk.yml":                         obeliskModuleYMLTemplate,
			filepath.Join(".obelisk", "dev.sh"):   moduleDevSHTemplate,
			filepath.Join(".obelisk", "build.sh"): moduleBuildSHTemplate,
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
		doneMsg = "Module initialized. Edit obelisk.yml and .obelisk/dev.sh to configure your module."
	} else {
		fmt.Println("Downloading template from github.com/nickdill/obelisk...")
		if err := applyTemplate(cwd, []string{"obelisk.yml", ".env"}, initForce); err != nil {
			return err
		}
		doneMsg = "Project initialized. Edit obelisk.yml to configure your modules."
	}

	// Ensure .env is listed in .gitignore (server projects only)
	if !initModuleMode {
		gitignorePath := filepath.Join(cwd, ".gitignore")
		if data, err := os.ReadFile(gitignorePath); err == nil {
			alreadyIgnored := false
			for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
				if strings.TrimSpace(line) == ".env" {
					alreadyIgnored = true
					break
				}
			}
			if !alreadyIgnored {
				updated := append(data, []byte("\n.env\n")...)
				if err := os.WriteFile(gitignorePath, updated, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
				} else {
					fmt.Println("  update .gitignore")
				}
			}
		} else if os.IsNotExist(err) {
			if err := os.WriteFile(gitignorePath, []byte(".env\n"), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not create .gitignore: %v\n", err)
			} else {
				fmt.Println("  create .gitignore")
			}
		}
	}

	if initForce && !initModuleMode {
		doneMsg = "Scripts updated to latest version. obelisk.yml was not changed."
	}
	fmt.Println("\n" + doneMsg)
	return nil
}

const obeliskModuleYMLTemplate = `version: "0.1"
name: "my-module"
type: module
# port: 3000
`

const moduleBuildSHTemplate = `#!/bin/sh
set -e
# Build this module for production.
# Replace the command below with your project's build command, for example:
#   npm run build
#   cargo build --release
#   go build -o bin/app .
echo "[Obelisk] No build command configured. Edit .obelisk/build.sh." >&2
exit 0
`

const moduleDevSHTemplate = `#!/bin/sh
set -e
# Start this module for local development.
# Replace the command below with your project's dev command, for example:
#   npm run dev
#   go run .
#   python -m flask run --port 3000
#   docker compose up
echo "[Obelisk] No dev command configured. Edit .obelisk/dev.sh to start this module." >&2
exit 0
`
