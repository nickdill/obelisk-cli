package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nickdill/obelisk/internal/config"
	"github.com/spf13/cobra"
)

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
