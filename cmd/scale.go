package cmd

import (
	"fmt"
	"strconv"

	"github.com/nickdill/obelisk/internal/client"
	"github.com/spf13/cobra"
)

var scaleServer string

var scaleCmd = &cobra.Command{
	Use:   "scale <module> <replicas>",
	Short: "Scale a module to N replicas on an Obelisk server",
	Args:  cobra.ExactArgs(2),
	RunE:  runScale,
}

func runScale(cmd *cobra.Command, args []string) error {
	module := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 0 {
		return fmt.Errorf("replicas must be a non-negative integer")
	}

	srv, err := resolveServer(scaleServer)
	if err != nil {
		return err
	}

	c := client.New(srv.URL)
	resp, err := c.Post("/v1/scale", map[string]any{"module": module, "replicas": n})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("scale failed (%d)", resp.StatusCode)
	}

	fmt.Printf("Scaled %s to %d replica(s) on %s.\n", module, n, srv.Name)
	return nil
}

func init() {
	scaleCmd.Flags().StringVar(&scaleServer, "server", "", "Target server name (required if multiple servers registered)")
	rootCmd.AddCommand(scaleCmd)
}
