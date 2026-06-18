package cmd

import (
	"bufio"
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

	base, existingTag := splitImageRef(cfg.Image)

	tag := publishTag
	if tag == "" {
		tag = existingTag
	}
	if tag == "" {
		tag = shortSHA()
	}
	if tag == "" {
		tag = "latest"
	}

	fullRef := base + ":" + tag
	fmt.Printf("Publishing %s\n\n", fullRef)

	// Build
	fmt.Println("==> Building image...")
	buildCmd := exec.Command("docker", "build", "-t", fullRef, ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Login — load registry credentials from the project .env
	if !publishSkipAuth {
		envVars := loadLocalEnv("REGISTRY_HOST", "REGISTRY_USER", "REGISTRY_TOKEN")
		registryHost := publishRegistry
		if registryHost == "" {
			registryHost = envVars["REGISTRY_HOST"]
		}
		if registryHost != "" {
			user := envVars["REGISTRY_USER"]
			token := envVars["REGISTRY_TOKEN"]
			if user == "" || token == "" {
				return fmt.Errorf("REGISTRY_USER and REGISTRY_TOKEN must be set in the project .env (or use --skip-login)")
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

// loadLocalEnv reads the .env file in the current directory and returns
// only the requested keys. Falls back to os.Getenv for each missing key.
func loadLocalEnv(keys ...string) map[string]string {
	result := make(map[string]string, len(keys))
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}

	if f, err := os.Open(".env"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			if want[k] {
				v = strings.TrimSpace(v)
				v = strings.Trim(v, "\"'")
				result[k] = v
			}
		}
	}

	for _, k := range keys {
		if result[k] == "" {
			result[k] = os.Getenv(k)
		}
	}
	return result
}

func splitImageRef(ref string) (base, tag string) {
	i := strings.LastIndex(ref, ":")
	if i > 0 && !strings.Contains(ref[i+1:], "/") {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}

func shortSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
