package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nickdill/obelisk/internal/config"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove all Obelisk-managed files from the current directory",
	RunE:  runUninstall,
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runUninstall(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	hasConfig := fileExists(filepath.Join(cwd, "obelisk.yml")) ||
		fileExists(filepath.Join(cwd, "obelisk.local.yml"))
	if !hasConfig {
		return fmt.Errorf("no Obelisk project found in current directory")
	}

	var targets []string
	if config.IsModule() {
		targets = []string{"obelisk.yml", ".obelisk"}
	} else {
		targets = []string{".obelisk", "docker-compose.yml", "docker-compose.override.yml", "obelisk.yml", "obelisk.local.yml"}
	}

	for _, target := range targets {
		full := filepath.Join(cwd, target)
		existed := fileExists(full)
		if err := os.RemoveAll(full); err != nil {
			return fmt.Errorf("could not remove %s: %w", target, err)
		}
		if existed {
			fmt.Printf("  remove   %s\n", target)
		}
	}

	fmt.Println("\nUninstalled. Run 'obelisk init' to reinitialize.")
	return nil
}
