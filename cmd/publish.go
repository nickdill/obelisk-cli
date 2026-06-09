package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish [module]",
	Short: "Build and push module images to a registry (coming soon)",
	RunE:  runPublish,
}

func runPublish(cmd *cobra.Command, args []string) error {
	fmt.Println("'obelisk publish' is coming soon.")
	fmt.Println("It will build and push your module images to ECR or any Docker registry.")
	return nil
}
