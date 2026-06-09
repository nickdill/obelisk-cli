package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const templateTarballURL = "https://codeload.github.com/nickdill/obelisk-template/tar.gz/refs/heads/main"

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new Obelisk project",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]
	base, err := os.Getwd()
	if err != nil {
		return err
	}
	projectDir := filepath.Join(base, name)

	fmt.Println("Downloading template from github.com/nickdill/obelisk-template...")

	resp, err := http.Get(templateTarballURL)
	if err != nil {
		return fmt.Errorf("could not download template: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not download template: HTTP %d", resp.StatusCode)
	}

	if err := extractTarGz(resp.Body, projectDir, name); err != nil {
		return fmt.Errorf("could not extract template: %w", err)
	}

	if err := setProjectName(projectDir, name); err != nil {
		return fmt.Errorf("could not set project name: %w", err)
	}

	fmt.Printf("\nProject '%s' created. Next steps:\n", name)
	fmt.Printf("  cd %s\n", name)
	fmt.Printf("  # Edit obelisk.yml to add your modules\n")
	fmt.Printf("  obelisk dev\n")
	return nil
}

func extractTarGz(r io.Reader, destDir, projectName string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		rel := parts[1]
		if strings.HasPrefix(rel, ".git/") || rel == ".git" {
			continue
		}

		full := filepath.Join(destDir, rel)
		if !strings.HasPrefix(full, destDir+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				return err
			}
			perm := os.FileMode(hdr.Mode).Perm()
			f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			fmt.Printf("  create %s/%s\n", projectName, rel)
		}
	}
	return nil
}

// setProjectName patches the name: field in obelisk.yml (or megalisk.yml as fallback
// while the template is being migrated).
func setProjectName(projectDir, name string) error {
	ymlPath := filepath.Join(projectDir, "obelisk.yml")
	if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
		ymlPath = filepath.Join(projectDir, "megalisk.yml")
		if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
			return nil
		}
	}

	data, err := os.ReadFile(ymlPath)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	cfg["name"] = name
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ymlPath, out, 0644)
}
