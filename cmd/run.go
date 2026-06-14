package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start all services in production mode (Docker Swarm)",
	RunE:  runRun,
}

func runRun(cmd *cobra.Command, args []string) error {
	script := ".obelisk/run.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return fmt.Errorf("no .obelisk/run.sh found — run 'obelisk init' first")
	}

	if err := checkGenerateComposeStale(); err != nil {
		return err
	}

	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
