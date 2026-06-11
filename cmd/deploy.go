package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/client"
	"github.com/nickdill/obelisk/internal/config"
)

var deployServer string

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
	if cfg.Type != "module" {
		return fmt.Errorf("deploy must be run from a module directory (type: module in obelisk.yml)")
	}

	srv, err := resolveServer(deployServer)
	if err != nil {
		return err
	}

	fmt.Printf("Deploying module %q to %s ...\n", cfg.Name, srv.Name)

	c := client.New(srv.URL)
	resp, err := c.Post("/v1/deploy", map[string]string{"module": cfg.Name})
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
}
