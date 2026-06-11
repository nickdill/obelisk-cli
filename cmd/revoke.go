package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/client"
)

var revokeServer string

var revokeCmd = &cobra.Command{
	Use:   "revoke <fingerprint>",
	Short: "Revoke a key from an Obelisk server",
	Long: `Remove an authorized key from an Obelisk server by its fingerprint.

Fingerprints are in SHA256:... format (from 'obelisk identity' or server key listings).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fingerprint := args[0]
		if !strings.HasPrefix(fingerprint, "SHA256:") {
			return fmt.Errorf("invalid fingerprint format — expected SHA256:... (from `obelisk identity`)")
		}

		srv, err := resolveServer(revokeServer)
		if err != nil {
			return err
		}

		// The fingerprint goes in the URL path; it uses URL-safe base64 so no escaping needed.
		agentPath := "/v1/keys/" + url.PathEscape(fingerprint)

		c := client.New(srv.URL)
		resp, err := c.Delete(agentPath)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode == 204 {
			fmt.Printf("Key %s revoked from %s.\n", fingerprint, srv.Name)
			return nil
		}

		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	},
}

func init() {
	revokeCmd.Flags().StringVar(&revokeServer, "server", "", "Target server name (required if multiple servers registered)")
}
