package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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

	if err := ensureSwarmManager(); err != nil {
		return err
	}

	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

// ensureSwarmManager is only called by obelisk run (Swarm mode), not obelisk dev (Compose).
func ensureSwarmManager() error {
	out, err := exec.Command("docker", "info", "--format", "{{.Swarm.LocalNodeState}}|{{.Swarm.ControlAvailable}}").Output()
	if err != nil {
		return fmt.Errorf("docker unavailable: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	state := parts[0]
	controlAvailable := len(parts) > 1 && parts[1] == "true"

	switch state {
	case "", "inactive":
		fmt.Println("[Obelisk] Initializing Docker Swarm...")
		c := exec.Command("docker", "swarm", "init")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	case "active":
		if !controlAvailable {
			return fmt.Errorf("this node is a Swarm worker, not a manager — run 'obelisk run' from the manager node")
		}
		return nil
	case "locked":
		return fmt.Errorf("Docker Swarm is locked — run 'docker swarm unlock' to restore access")
	case "error":
		return fmt.Errorf("Docker Swarm is in an error state — check 'docker info' for details")
	default:
		return fmt.Errorf("unexpected Docker Swarm state %q — check 'docker info'", state)
	}
}
