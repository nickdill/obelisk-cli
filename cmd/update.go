package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var updateHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		ForceAttemptHTTP2: false,
	},
}

const githubRepo = "nickdill/obelisk-cli"

var updateCmd = &cobra.Command{
	Use:   "update [version]",
	Short: "Update the obelisk CLI to the latest version",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	var targetVersion string

	if len(args) == 1 {
		targetVersion = args[0]
		if !strings.HasPrefix(targetVersion, "v") {
			targetVersion = "v" + targetVersion
		}
		fmt.Printf("Checking for version %s...\n", targetVersion)
		if err := validateRelease(targetVersion); err != nil {
			return err
		}
	} else {
		fmt.Println("Checking for updates...")
		v, err := fetchLatestVersion()
		if err != nil {
			return err
		}
		targetVersion = v
	}

	if version == targetVersion {
		fmt.Printf("Already up to date (%s).\n", version)
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate current binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("could not resolve binary path: %w", err)
	}

	asset := fmt.Sprintf("obelisk-%s-%s", runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, targetVersion, asset)

	fmt.Printf("Updating obelisk %s → %s...\n", version, targetVersion)
	fmt.Printf("Downloading %s...\n", asset)

	tmpFile, err := os.CreateTemp(filepath.Dir(execPath), ".obelisk-update-*")
	if err != nil {
		return fmt.Errorf("could not create temp file (check write permissions for %s): %w", filepath.Dir(execPath), err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	resp, err := updateHTTPClient.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: server returned %s for %s", resp.Status, url)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("could not set permissions: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("could not replace binary (try running with sudo): %w", err)
	}

	fmt.Printf("obelisk updated to %s\n", targetVersion)
	return nil
}

func fetchLatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := updateHTTPClient.Get(url) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("no releases found for %s — push a git tag to publish one", githubRepo)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("could not parse GitHub response: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("could not determine latest version — check your internet connection")
	}
	return release.TagName, nil
}

func validateRelease(version string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", githubRepo, version)
	resp, err := updateHTTPClient.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("release %s not found", version)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	return nil
}
