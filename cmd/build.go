package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Run the module build script (.obelisk/build.sh)",
	RunE:  runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	script := ".obelisk/build.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return fmt.Errorf("no .obelisk/build.sh found — run 'obelisk init --module' first")
	}
	c := exec.Command("sh", script)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
