package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop all services",
	RunE:  runDown,
}

func runDown(cmd *cobra.Command, args []string) error {
	c := exec.Command("docker", "stack", "rm", "obelisk")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
