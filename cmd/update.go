package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var sourceDir string

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the obelisk CLI to the latest version",
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	if sourceDir == "" {
		fmt.Println("To update obelisk, re-run the installer:")
		fmt.Println("  curl -fsSL https://raw.githubusercontent.com/nickdill/obelisk/main/install.sh | bash")
		return nil
	}

	script := filepath.Join(sourceDir, "install-local.sh")
	if _, err := os.Stat(script); os.IsNotExist(err) {
		fmt.Println("To update obelisk, re-run the installer:")
		fmt.Println("  curl -fsSL https://raw.githubusercontent.com/nickdill/obelisk/main/install.sh | bash")
		return nil
	}

	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
