package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/nickdill/obelisk/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of the current project",
	RunE:  runStatus,
}

type containerInfo struct {
	State  string
	Health string
}

func runStatus(cmd *cobra.Command, args []string) error {
	_, initErr := os.Stat(".obelisk")
	initialized := initErr == nil

	if config.IsModule() {
		modCfg, err := config.LoadModule()
		if err != nil {
			return err
		}
		return showModuleStatus(modCfg, initialized)
	}

	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No obelisk.yml found in this directory.")
			fmt.Println("Run `obelisk init` to set up a server, or `obelisk init --module` to set up a module.")
			return nil
		}
		return err
	}
	return showServerStatus(cfg, initialized)
}

func showModuleStatus(cfg *config.ModuleConfig, initialized bool) error {
	fmt.Printf("Module:  %s\n", cfg.Name)
	if initialized {
		fmt.Println("Init:    ✓ initialized")
	} else {
		fmt.Println("Init:    ✗ not initialized — run obelisk init --module")
	}
	if cfg.Port != 0 {
		fmt.Printf("Port:    %d\n", cfg.Port)
	}
	return nil
}

func showServerStatus(cfg *config.Config, initialized bool) error {
	fmt.Printf("Project:  %s  (server)\n", cfg.Name)
	if initialized {
		fmt.Println("Init:     ✓ initialized")
	} else {
		fmt.Println("Init:     ✗ not initialized — run obelisk init")
	}
	fmt.Println()

	containers, dcErr := dockerComposePS()
	names := sortedModuleKeys(cfg.Modules)

	nameWidth := len("MODULE")
	for _, n := range names {
		if len(n) > nameWidth {
			nameWidth = len(n)
		}
	}

	fmt.Printf("%-*s   %-10s   %s\n", nameWidth, "MODULE", "STATE", "HEALTH")
	for _, name := range names {
		state := "—"
		health := ""
		if containers != nil {
			if info, ok := containers[name]; ok {
				state = info.State
				health = info.Health
			}
		}
		fmt.Printf("%-*s   %-10s   %s\n", nameWidth, name, state, health)
	}

	if dcErr != nil {
		fmt.Printf("\nnote: could not reach docker compose (%v)\n", dcErr)
	}
	return nil
}

func dockerComposePS() (map[string]containerInfo, error) {
	out, err := exec.Command("docker", "compose", "ps", "--format", "json").Output()
	if err != nil {
		return nil, err
	}

	type row struct {
		Service string `json:"Service"`
		State   string `json:"State"`
		Health  string `json:"Health"`
	}

	result := make(map[string]containerInfo)

	// Docker Compose v2.17+ outputs a JSON array; older versions output NDJSON.
	var rows []row
	if err := json.Unmarshal(out, &rows); err == nil {
		for _, r := range rows {
			result[r.Service] = containerInfo{State: r.State, Health: r.Health}
		}
		return result, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r row
		if json.Unmarshal(line, &r) == nil {
			result[r.Service] = containerInfo{State: r.State, Health: r.Health}
		}
	}
	return result, nil
}

func sortedModuleKeys(m map[string]*config.Module) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
