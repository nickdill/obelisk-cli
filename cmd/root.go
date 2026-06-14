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
   /________\   \___/ |____/ |_____||_____||___| |____/ |_|\_\`

var rootCmd = &cobra.Command{
	Use:     "obelisk",
	Short:   "Obelisk — deploy multiple projects to one server",
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(banner)
		fmt.Println()
		fmt.Println("Common commands:")
		rows := [][4]string{
			{"init", "create a server project", "init -m", "create a module"},
			{"dev", "run local dev", "dev --build", "build & run local"},
			{"run", "start production", "stop", "stop services"},
			{"deploy", "deploy a module", "list", "status all servers"},
			{"server add", "add a server", "identity", "show your key"},
		}
		for _, r := range rows {
			fmt.Printf("  \033[1m%-16s\033[0m%-29s\033[1m%-15s\033[0m%s\n", r[0], r[1], r[2], r[3])
		}
		fmt.Println()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate("Obelisk CLI version {{.Version}}\n")
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(stopCmd)
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
