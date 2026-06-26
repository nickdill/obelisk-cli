package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/client"
	"github.com/nickdill/obelisk/internal/config"
)

var deployServer string
var deployImage string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the current module to an Obelisk server",
	RunE:  runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading obelisk.yml: %w", err)
	}
	if cfg.Type != "module" && cfg.Type != "static" {
		return fmt.Errorf("deploy must be run from a module or static directory (type: module|static in obelisk.yml)")
	}

	srv, err := resolveServer(deployServer)
	if err != nil {
		return err
	}

	fmt.Printf("Deploying module %q to %s ...\n", cfg.Name, srv.Name)

	deployBody := map[string]any{"module": cfg.Name}
	if cfg.Type == "static" {
		// Static modules are delivered as an artifact image the server pulls and
		// extracts into the shared volume. Tell the agent so, and resolve the
		// exact image ref the same way `obelisk publish` tags it.
		if cfg.Image == "" {
			return fmt.Errorf("no 'image' field in obelisk.yml — static modules must declare a registry path to deploy")
		}
		deployBody["static"] = true
		img := deployImage
		if img == "" {
			base, existingTag := splitImageRef(cfg.Image)
			tag := existingTag
			if tag == "" {
				tag = shortSHA()
			}
			if tag == "" {
				tag = "latest"
			}
			img = base + ":" + tag
		}
		deployBody["image"] = img
	} else if deployImage != "" {
		deployBody["image"] = deployImage
	}
	if sha := gitSHA(); sha != "" {
		deployBody["sha"] = sha
	}
	if branch := gitBranch(); branch != "" {
		deployBody["branch"] = branch
	}
	if tag := gitTag(); tag != "" {
		deployBody["tag"] = tag
	}

	c := client.New(srv.URL)
	resp, err := c.Post("/v1/deploy", deployBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("a deploy is already in progress on %s", srv.Name)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deploy failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	exitCode, err := streamDeploy(resp.Body)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// streamDeploy reads the streaming deploy response, printing each line as it
// arrives. The final line is a JSON object {"exit_code": N}; it returns that
// value rather than printing it.
func streamDeploy(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	var lastLine string
	for scanner.Scan() {
		line := scanner.Text()
		// Buffer lines; print the previous one only when we see a next one,
		// so we can identify and suppress the trailing JSON result line.
		if lastLine != "" {
			fmt.Println(lastLine)
		}
		lastLine = line
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("reading deploy output: %w", err)
	}

	// lastLine should be the JSON result
	var result struct {
		ExitCode int `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(lastLine), &result); err != nil {
		// Not parseable as JSON — print it and assume failure
		if lastLine != "" {
			fmt.Println(lastLine)
		}
		return 1, fmt.Errorf("unexpected end of deploy stream")
	}

	if result.ExitCode == 0 {
		fmt.Println("Deploy complete.")
	} else {
		fmt.Fprintf(os.Stderr, "Deploy failed (exit code %d).\n", result.ExitCode)
	}
	return result.ExitCode, nil
}

func init() {
	deployCmd.Flags().StringVar(&deployServer, "server", "", "Target server name (required if multiple servers registered)")
	deployCmd.Flags().StringVar(&deployImage, "image", "", "Exact image ref to deploy (e.g. ghcr.io/user/mod:abc123)")
}

func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "" // detached HEAD
	}
	return branch
}

func gitTag() string {
	out, err := exec.Command("git", "describe", "--tags", "--exact-match").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
