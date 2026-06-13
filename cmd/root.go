package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

const banner = `
       /\        ___   ____   _____  _      ___   ____  _  __
      /  \      / _ \ | __ ) | ____|| |    |_ _| / ___|| |/ /
     /    \    | | | ||  _ \ |  _|  | |     | |  \___ \| ' /
    /      \   | |_| || |_) || |___ | |___  | |   ___) || . \
   /________\   \___/ |____/ |_____||_____||___| |____/ |_|\_\
   Deploy multiple projects to one server`

var rootCmd = &cobra.Command{
	Use:     "obelisk",
	Short:   "Obelisk — deploy multiple projects to one server",
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(banner)
		fmt.Println()
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(identityCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(allowCmd)
	rootCmd.AddCommand(revokeCmd)
	rootCmd.AddCommand(listCmd)
}
