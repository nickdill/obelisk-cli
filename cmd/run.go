package cmd

import (
	"fmt"
	"io"
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

	if err := checkYQ(); err != nil {
		return err
	}

	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func checkYQ() error {
	if _, err := exec.LookPath("yq"); err != nil {
		return fmt.Errorf("yq is required but not installed\n\nInstall it with:\n  brew install yq           # macOS / Linuxbrew\n  snap install yq           # Ubuntu/Debian\n  pip install yq            # Python (any platform)\n  https://github.com/mikefarah/yq/releases  # manual download")
	}
	return nil
}

func checkDockerAccess() error {
	out, err := exec.Command("docker", "info").CombinedOutput()
	if err != nil && strings.Contains(string(out), "permission denied") &&
		strings.Contains(string(out), "docker.sock") {
		return fmt.Errorf("permission denied: cannot connect to the Docker daemon\nAdd your user to the docker group and re-login:\n\n  sudo usermod -aG docker $USER\n  newgrp docker   # or log out and back in")
	}
	return nil
}

// ensureSwarmManager is only called by obelisk run (Swarm mode), not obelisk dev (Compose).
func ensureSwarmManager() error {
	if err := checkDockerAccess(); err != nil {
		return err
	}

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
		var stderrBuf strings.Builder
		c := exec.Command("docker", "swarm", "init")
		c.Stdout = os.Stdout
		c.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
		if err := c.Run(); err != nil {
			if strings.Contains(stderrBuf.String(), "permission denied") &&
				strings.Contains(stderrBuf.String(), "docker.sock") {
				return fmt.Errorf("permission denied: cannot connect to the Docker daemon\nAdd your user to the docker group and re-login:\n\n  sudo usermod -aG docker $USER\n  newgrp docker   # or log out and back in")
			}
			return err
		}
		return nil
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
