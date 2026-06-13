package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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

	if err := applyTemplate(projectDir, nil, false); err != nil {
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
