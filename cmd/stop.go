package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop all running services",
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	script := ".obelisk/stop.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return fmt.Errorf("no .obelisk/stop.sh found — run 'obelisk init' first")
	}

	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
