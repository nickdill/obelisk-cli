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
	publishPlatform string
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
	publishCmd.Flags().StringVar(&publishPlatform, "platform", "", "Target platform for the image build (default: linux/amd64)")
}

func runPublish(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading obelisk.yml: %w", err)
	}
	if cfg.Type != "module" && cfg.Type != "static" {
		return fmt.Errorf("publish must be run from a module or static directory (type: module|static in obelisk.yml)")
	}
	if cfg.Image == "" {
		return fmt.Errorf("no 'image' field in obelisk.yml — set it to your registry path, e.g. ghcr.io/<user>/%s", cfg.Name)
	}

	platform := publishPlatform
	if platform == "" {
		platform = cfg.Platform
	}
	if platform == "" {
		platform = "linux/amd64"
	}
	platform = normalizePlatform(platform)
	if err := validatePlatform(platform); err != nil {
		return err
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
				return fmt.Errorf("REGISTRY_USER and REGISTRY_TOKEN must be set in .env (or use --skip-login)")
			}
			fmt.Printf("==> Logging in to %s...\n", registryHost)
			loginCmd := exec.Command("docker", "login", registryHost, "--username", user, "--password-stdin")
			loginCmd.Stdin = strings.NewReader(token)
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			if err := loginCmd.Run(); err != nil {
				return fmt.Errorf("docker login failed: %w", err)
			}
		}
	}

	// Static modules: build assets locally with the project's own toolchain,
	// then package the output into a throwaway busybox "artifact image" whose
	// /static dir the server extracts into the shared volume. Modules keep using
	// their own Dockerfile.
	var dockerfile string
	if cfg.Type == "static" {
		if cfg.Build != "" {
			fmt.Printf("==> Building static assets: %s\n", cfg.Build)
			b := exec.Command("sh", "-c", cfg.Build)
			b.Stdout = os.Stdout
			b.Stderr = os.Stderr
			if err := b.Run(); err != nil {
				return fmt.Errorf("static build command failed: %w", err)
			}
		}
		dist := cfg.Dist
		if dist == "" {
			dist = "."
		}
		df, cleanup, err := writeStaticDockerfile(dist)
		if err != nil {
			return err
		}
		defer cleanup()
		dockerfile = df
	}

	// Multi-platform builds require a builder with the docker-container driver.
	// The default "docker" driver only supports single-platform.
	buildArgs := []string{"buildx", "build", "--platform", platform, "-t", fullRef, "--push"}
	if dockerfile != "" {
		buildArgs = append(buildArgs, "-f", dockerfile)
	}
	if strings.Contains(platform, ",") {
		builder, err := ensureMultiPlatformBuilder()
		if err != nil {
			return err
		}
		buildArgs = append(buildArgs, "--builder", builder)
	}
	buildArgs = append(buildArgs, ".")

	fmt.Printf("\n==> Building for %s and pushing...\n", platform)
	buildCmd := exec.Command("docker", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker buildx build+push failed: %w", err)
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

// writeStaticDockerfile writes a throwaway Dockerfile that packages a static
// module's built assets into a minimal busybox image at /static. busybox gives
// the server a shell + cp/mv for the extract step in sync-static.sh.
func writeStaticDockerfile(dist string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "obelisk-static-*.Dockerfile")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp Dockerfile: %w", err)
	}
	content := fmt.Sprintf("FROM busybox\nCOPY %s /static\n", dist)
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("writing temp Dockerfile: %w", err)
	}
	f.Close()
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

func splitImageRef(ref string) (base, tag string) {
	i := strings.LastIndex(ref, ":")
	if i > 0 && !strings.Contains(ref[i+1:], "/") {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}

func ensureMultiPlatformBuilder() (string, error) {
	name := "obelisk"
	out, err := exec.Command("docker", "buildx", "inspect", name).CombinedOutput()
	if err == nil && strings.Contains(string(out), "docker-container") {
		return name, nil
	}
	fmt.Println("==> Creating multi-platform builder...")
	create := exec.Command("docker", "buildx", "create", "--name", name, "--driver", "docker-container")
	create.Stdout = os.Stdout
	create.Stderr = os.Stderr
	if err := create.Run(); err != nil {
		return "", fmt.Errorf("failed to create buildx builder: %w\nSee https://docs.docker.com/go/build-multi-platform/", err)
	}
	return name, nil
}

func normalizePlatform(platform string) string {
	parts := strings.Split(platform, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, ",")
}

func validatePlatform(platform string) error {
	for _, p := range strings.Split(platform, ",") {
		parts := strings.SplitN(p, "/", 3)
		if len(parts) < 2 {
			return fmt.Errorf("invalid platform %q: expected os/arch (e.g. linux/amd64)", p)
		}
	}
	return nil
}

func shortSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
