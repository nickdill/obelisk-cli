package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy to production (coming soon)",
	RunE:  runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
	fmt.Println("'obelisk deploy' is coming soon.")
	fmt.Println("For now, deploy using AWS CodeDeploy (appspec.yml) or SSH manually.")
	return nil
}
