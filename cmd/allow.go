package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/client"
	"github.com/nickdill/obelisk/internal/registry"
)

var allowName   string
var allowServer string

var allowCmd = &cobra.Command{
	Use:   "allow <pubkey>",
	Short: "Authorize a key on an Obelisk server",
	Long: `Push a teammate's public key to a server so they can run obelisk commands.

The pubkey argument should be in obk1_... format (from 'obelisk identity').`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pubKey := args[0]
		if !strings.HasPrefix(pubKey, "obk1_") {
			return fmt.Errorf("invalid public key format — expected obk1_... (from `obelisk identity`)")
		}

		srv, err := resolveServer(allowServer)
		if err != nil {
			return err
		}

		name := allowName
		if name == "" {
			name = promptLabel(pubKey)
		}

		c := client.New(srv.URL)
		resp, err := c.Post("/v1/keys", map[string]string{
			"key":  pubKey,
			"name": name,
		})
		if err != nil {
			return err
		}

		var result struct {
			Fingerprint string `json:"fingerprint"`
		}
		if err := client.DecodeJSON(resp, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		fmt.Printf("Key authorized on %s.\n", srv.Name)
		fmt.Printf("  Name:        %s\n", name)
		fmt.Printf("  Fingerprint: %s\n", result.Fingerprint)
		return nil
	},
}

func promptLabel(pubKey string) string {
	fmt.Printf("Name for key %s...%s (optional): ", pubKey[:10], pubKey[len(pubKey)-4:])
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		label := strings.TrimSpace(scanner.Text())
		if label != "" {
			return label
		}
	}
	return pubKey[:16]
}

func resolveServer(name string) (registry.Server, error) {
	return registry.Resolve(name)
}

func init() {
	allowCmd.Flags().StringVar(&allowName, "name", "", "Label for the key (e.g. teammate's name)")
	allowCmd.Flags().StringVar(&allowServer, "server", "", "Target server name (required if multiple servers registered)")
}
