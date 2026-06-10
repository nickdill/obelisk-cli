package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var uninstallPurge bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Obelisk-managed files from the current directory",
	RunE:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallPurge, "purge", false, "Also remove obelisk.yml")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	dirs := []string{".obelisk"}
	files := []string{"docker-compose.yml", "docker-compose.override.yml"}

	for _, d := range dirs {
		full := filepath.Join(cwd, d)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			continue
		}
		if err := os.RemoveAll(full); err != nil {
			return fmt.Errorf("could not remove %s: %w", d, err)
		}
		fmt.Printf("  remove   %s/\n", d)
	}

	for _, f := range files {
		full := filepath.Join(cwd, f)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			continue
		}
		if err := os.Remove(full); err != nil {
			return fmt.Errorf("could not remove %s: %w", f, err)
		}
		fmt.Printf("  remove   %s\n", f)
	}

	if uninstallPurge {
		full := filepath.Join(cwd, "obelisk.yml")
		if _, err := os.Stat(full); err == nil {
			if err := os.Remove(full); err != nil {
				return fmt.Errorf("could not remove obelisk.yml: %w", err)
			}
			fmt.Println("  remove   obelisk.yml")
		}
	} else {
		fmt.Println("  skip     obelisk.yml  (use --purge to also remove config)")
	}

	fmt.Println("\nUninstalled. Run 'obelisk init' to reinitialize.")
	return nil
}
