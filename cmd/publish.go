package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/config"
)

var (
	publishTag      string
	publishRegistry string
	publishSkipAuth bool
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Build and push a module image to a container registry",
	Long: `Build a Docker image for the current module and push it to a container registry.

The image path is read from the 'image' field in obelisk.yml. The tag defaults
to the current git SHA, or 'latest' if not in a git repo.

Examples:
  obelisk publish
  obelisk publish --tag v1.2.0
  obelisk publish --registry ghcr.io --tag latest`,
	RunE: runPublish,
}

func init() {
	publishCmd.Flags().StringVar(&publishTag, "tag", "", "Image tag (default: git SHA or 'latest')")
	publishCmd.Flags().StringVar(&publishRegistry, "registry", "", "Registry host to login to (overrides REGISTRY_HOST env)")
	publishCmd.Flags().BoolVar(&publishSkipAuth, "skip-login", false, "Skip docker login (use when auth is handled externally)")
}

func runPublish(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading obelisk.yml: %w", err)
	}
	if cfg.Type != "module" {
		return fmt.Errorf("publish must be run from a module directory (type: module in obelisk.yml)")
	}
	if cfg.Image == "" {
		return fmt.Errorf("no 'image' field in obelisk.yml — set it to your registry path, e.g. ghcr.io/<user>/%s", cfg.Name)
	}

	tag := publishTag
	if tag == "" {
		tag = shortSHA()
	}
	if tag == "" {
		tag = "latest"
	}

	fullRef := cfg.Image + ":" + tag
	fmt.Printf("Publishing %s\n\n", fullRef)

	// Build
	fmt.Println("==> Building image...")
	buildCmd := exec.Command("docker", "build", "-t", fullRef, ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Login
	if !publishSkipAuth {
		registryHost := publishRegistry
		if registryHost == "" {
			registryHost = os.Getenv("REGISTRY_HOST")
		}
		if registryHost != "" {
			user := os.Getenv("REGISTRY_USER")
			token := os.Getenv("REGISTRY_TOKEN")
			if user == "" || token == "" {
				return fmt.Errorf("REGISTRY_USER and REGISTRY_TOKEN must be set (or use --skip-login)")
			}
			fmt.Printf("\n==> Logging in to %s...\n", registryHost)
			loginCmd := exec.Command("docker", "login", registryHost, "--username", user, "--password-stdin")
			loginCmd.Stdin = strings.NewReader(token)
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			if err := loginCmd.Run(); err != nil {
				return fmt.Errorf("docker login failed: %w", err)
			}
		}
	}

	// Push
	fmt.Println("\n==> Pushing image...")
	pushCmd := exec.Command("docker", "push", fullRef)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}

	fmt.Printf("\nPublished: %s\n", fullRef)
	return nil
}

func shortSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
