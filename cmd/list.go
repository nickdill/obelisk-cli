package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/nickdill/obelisk/internal/client"
	"github.com/nickdill/obelisk/internal/registry"
)

type moduleStatus struct {
	Server string
	URL    string
	Name   string
	State  string
	Health string
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show module status across all registered servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := registry.List()
		if err != nil {
			return err
		}
		if len(servers) == 0 {
			fmt.Println("No servers registered. Run `obelisk server add <name> <url>` to add one.")
			return nil
		}

		type result struct {
			modules []moduleStatus
			err     error
		}

		results := make([]result, len(servers))
		var wg sync.WaitGroup
		for i, srv := range servers {
			wg.Add(1)
			go func(i int, srv registry.Server) {
				defer wg.Done()
				modules, err := fetchStatus(srv)
				results[i] = result{modules: modules, err: err}
			}(i, srv)
		}
		wg.Wait()

		var rows []moduleStatus
		var errs []string
		for i, r := range results {
			if r.err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", servers[i].Name, r.err))
				continue
			}
			if len(r.modules) == 0 {
				rows = append(rows, moduleStatus{
					Server: servers[i].Name,
					URL:    servers[i].URL,
					Name:   "(no modules)",
					State:  "-",
					Health: "-",
				})
				continue
			}
			rows = append(rows, r.modules...)
		}

		if len(rows) == 0 && len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintln(cmd.ErrOrStderr(), "error:", e)
			}
			return fmt.Errorf("could not reach any servers")
		}

		fmt.Printf("%-20s  %-30s  %-20s  %-10s  %s\n", "SERVER", "URL", "MODULE", "STATE", "HEALTH")
		fmt.Printf("%-20s  %-30s  %-20s  %-10s  %s\n",
			strings.Repeat("-", 20), strings.Repeat("-", 30),
			strings.Repeat("-", 20), strings.Repeat("-", 10), strings.Repeat("-", 10))
		for _, row := range rows {
			fmt.Printf("%-20s  %-30s  %-20s  %-10s  %s\n", row.Server, row.URL, row.Name, row.State, row.Health)
		}

		for _, e := range errs {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", e)
		}
		return nil
	},
}

func fetchStatus(srv registry.Server) ([]moduleStatus, error) {
	c := client.New(srv.URL)
	resp, err := c.Get("/v1/status")
	if err != nil {
		return nil, err
	}

	var body struct {
		Modules []struct {
			Name    string `json:"name"`
			Service string `json:"service"`
			State   string `json:"state"`
			Health  string `json:"health"`
		} `json:"modules"`
	}
	if err := client.DecodeJSON(resp, &body); err != nil {
		return nil, err
	}

	modules := make([]moduleStatus, 0, len(body.Modules))
	for _, m := range body.Modules {
		modules = append(modules, moduleStatus{
			Server: srv.Name,
			URL:    srv.URL,
			Name:   m.Name,
			State:  m.State,
			Health: m.Health,
		})
	}
	return modules, nil
}
