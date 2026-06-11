package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/identity"
)

var identityForce bool

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Show (or generate) your local identity key",
	Long: `Print your public key and fingerprint. Generates a keypair on first run.

Send your public key to a server admin so they can run:
  obelisk allow <pubkey> --server <name>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if identityForce {
			if err := identity.Generate(true); err != nil {
				return err
			}
			fmt.Println("New keypair generated.")
		}

		pubKey, err := identity.PublicKeyString()
		if err != nil {
			return err
		}
		fp, err := identity.Fingerprint()
		if err != nil {
			return err
		}

		fmt.Printf("Public key:  %s\n", pubKey)
		fmt.Printf("Fingerprint: %s\n", fp)
		fmt.Println()
		fmt.Println("Send your public key to a server admin to get access.")
		return nil
	},
}

func init() {
	identityCmd.Flags().BoolVar(&identityForce, "force", false, "Overwrite existing keypair")
}
