package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [service...]",
	Short: "Tail logs for all services (or a specific one)",
	RunE:  runLogs,
}

func runLogs(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat("docker-compose.yml"); os.IsNotExist(err) {
		return fmt.Errorf("no docker-compose.yml found — run 'obelisk init' or 'obelisk new' first")
	}

	cmdArgs := append([]string{"compose", "logs", "-f"}, args...)
	c := exec.Command("docker", cmdArgs...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
