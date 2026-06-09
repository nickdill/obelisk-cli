package cmd

import (
	"fmt"
	"os"

	"github.com/nickdill/obelisk/internal/config"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Print the active Obelisk config",
	RunE:  runDebug,
}

func runDebug(cmd *cobra.Command, args []string) error {
	path := config.Path()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", path, err)
	}
	fmt.Printf("Config: %s\n\n", path)
	fmt.Print(string(data))
	return nil
}
