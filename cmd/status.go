package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show health status for all services (coming soon)",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("'obelisk status' is coming soon.")
	fmt.Println("For now, use: docker compose ps")
	return nil
}
