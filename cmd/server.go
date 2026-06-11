package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/client"
	"github.com/nickdill/obelisk/internal/registry"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage registered Obelisk servers",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var serverAddCmd = &cobra.Command{
	Use:   "add <name> <url>",
	Short: "Register a server and verify connectivity",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, rawURL := args[0], args[1]

		// Normalize URL
		if !strings.Contains(rawURL, "://") {
			rawURL = "https://" + rawURL
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		serverURL := strings.TrimRight(u.String(), "/")

		fmt.Printf("Connecting to %s ...\n", serverURL)

		c := client.New(serverURL)
		resp, err := c.Get("/v1/ping")
		if err != nil {
			return err
		}

		var ping struct {
			AgentVersion string `json:"agent_version"`
			ServerName   string `json:"server_name"`
			Protocol     string `json:"protocol"`
		}
		if err := client.DecodeJSON(resp, &ping); err != nil {
			return fmt.Errorf("parsing ping response: %w", err)
		}

		if err := registry.Add(name, serverURL); err != nil {
			return err
		}

		fmt.Printf("Server %q registered.\n", name)
		if ping.ServerName != "" {
			fmt.Printf("  Server name:   %s\n", ping.ServerName)
		}
		if ping.AgentVersion != "" {
			fmt.Printf("  Agent version: %s\n", ping.AgentVersion)
		}
		return nil
	},
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := registry.List()
		if err != nil {
			return err
		}
		if len(servers) == 0 {
			fmt.Println("No servers registered. Run `obelisk server add <name> <url>` to add one.")
			return nil
		}

		fmt.Printf("%-20s  %-45s  %s\n", "NAME", "URL", "LAST SEEN")
		fmt.Printf("%-20s  %-45s  %s\n", strings.Repeat("-", 20), strings.Repeat("-", 45), strings.Repeat("-", 20))
		for _, s := range servers {
			lastSeen := "never"
			if !s.LastSeen.IsZero() {
				lastSeen = s.LastSeen.Format("2006-01-02 15:04 UTC")
			}
			fmt.Printf("%-20s  %-45s  %s\n", s.Name, s.URL, lastSeen)
		}
		return nil
	},
}

var serverRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := registry.Remove(args[0]); err != nil {
			return err
		}
		fmt.Printf("Server %q removed.\n", args[0])
		return nil
	},
}

func init() {
	serverCmd.AddCommand(serverAddCmd)
	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverRemoveCmd)
}
