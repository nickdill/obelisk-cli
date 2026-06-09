package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start all services in development mode",
	RunE:  runDev,
}

func runDev(cmd *cobra.Command, args []string) error {
	script := ".obelisk/dev.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		script = ".obelisk/run.sh"
		if _, err := os.Stat(script); os.IsNotExist(err) {
			return fmt.Errorf("no .obelisk/dev.sh found — run 'obelisk init' first")
		}
	}

	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
