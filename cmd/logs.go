package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <module>",
	Short: "Tail logs for a module",
	RunE:  runLogs,
}

func runLogs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a module name: obelisk logs <module>")
	}
	service := "obelisk_" + args[0]
	c := exec.Command("docker", "service", "logs", "--follow", service)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
