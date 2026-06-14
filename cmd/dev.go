package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nickdill/obelisk/internal/config"
	"github.com/spf13/cobra"
)

const swarmStackName = "obelisk"
const swarmComposeNetwork = "obelisk"                                    // network name defined in the template's compose file
const swarmNetworkName = swarmStackName + "_" + swarmComposeNetwork // Docker prefixes it with the stack name on deploy

var devBuild bool

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start all services in development mode",
	RunE:  runDev,
}

func init() {
	devCmd.Flags().BoolVar(&devBuild, "build", false, "Build images before starting")
}

func runDev(cmd *cobra.Command, args []string) error {
	script := ".obelisk/dev.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		// run.sh fallback for projects initialized before dev.sh was added
		script = ".obelisk/run.sh"
		if _, err := os.Stat(script); os.IsNotExist(err) {
			return fmt.Errorf("no .obelisk/dev.sh found — run 'obelisk init' (server) or 'obelisk init --module' (module) first")
		}
	}

	if err := checkGenerateComposeStale(); err != nil {
		return err
	}

	if err := cleanupSwarmNetworks(); err != nil {
		return err
	}

	if err := checkYQ(); err != nil {
		return err
	}

	if devBuild {
		build := exec.Command("docker", "compose", "build")
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			return fmt.Errorf("docker compose build failed: %w", err)
		}
	}

	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

// cleanupSwarmNetworks tears down a running Swarm stack and removes the
// obelisk_obelisk overlay network before Compose starts. Compose refuses to
// reuse Swarm-owned networks because their labels don't match. Stack removal
// is async, so we poll until the network is gone or removable.
func cleanupSwarmNetworks() error {
	out, err := exec.Command("docker", "network", "inspect", swarmNetworkName, "--format", "{{.Driver}}").Output()
	if err != nil {
		return nil // network doesn't exist — nothing to do
	}
	if strings.TrimSpace(string(out)) != "overlay" {
		return nil // not a Swarm network; Compose will handle it normally
	}

	fmt.Println("[Obelisk] Stopping Swarm stack...")
	rmStack := exec.Command("docker", "stack", "rm", swarmStackName)
	rmStack.Stderr = os.Stderr // surface unexpected errors; "not found" is fine to ignore
	rmStack.Run()

	fmt.Print("[Obelisk] Waiting for network to be released")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		fmt.Print(".")
		if exec.Command("docker", "network", "rm", swarmNetworkName).Run() == nil {
			fmt.Println()
			return nil
		}
		// Network may have disappeared on its own between the rm attempt and here
		if _, err := exec.Command("docker", "network", "inspect", swarmNetworkName, "--format", "{{.ID}}").Output(); err != nil {
			fmt.Println()
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Println()
	return fmt.Errorf("timed out waiting for Swarm network to be released — try again in a moment")
}

// checkGenerateComposeStale warns when any module uses git_source but the
// installed generate-compose.sh predates git_source support.
func checkGenerateComposeStale() error {
	if config.IsModule() {
		return nil // module projects don't use generate-compose.sh
	}

	cfg, err := config.Load()
	if err != nil {
		return nil // no server config present — nothing to validate
	}

	needsGitSource := false
	for _, m := range cfg.Modules {
		if m.GitSource != "" {
			needsGitSource = true
			break
		}
	}
	if !needsGitSource {
		return nil
	}

	data, err := os.ReadFile(".obelisk/scripts/generate-compose.sh")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !strings.Contains(string(data), "git_source") {
		return fmt.Errorf(
			"one or more modules use git_source but .obelisk/scripts/generate-compose.sh\n" +
				"is from an older version that does not support it.\n" +
				"Run: obelisk init --force",
		)
	}
	return nil
}
