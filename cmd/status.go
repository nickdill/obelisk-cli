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
	"strings"

	"github.com/nickdill/obelisk/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of the current project",
	RunE:  runStatus,
}

type containerInfo struct {
	State    string
	Replicas string
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

	services, svcErr := dockerSwarmServices()
	names := sortedModuleKeys(cfg.Modules)

	nameWidth := len("MODULE")
	for _, n := range names {
		if len(n) > nameWidth {
			nameWidth = len(n)
		}
	}

	fmt.Printf("%-*s   %-10s   %s\n", nameWidth, "MODULE", "STATE", "REPLICAS")
	for _, name := range names {
		state := "—"
		replicas := ""
		if services != nil {
			if info, ok := services[name]; ok {
				state = info.State
				replicas = info.Replicas
			}
		}
		fmt.Printf("%-*s   %-10s   %s\n", nameWidth, name, state, replicas)
	}

	if svcErr != nil {
		fmt.Printf("\nnote: could not reach docker swarm (%v)\n", svcErr)
	}
	return nil
}

func dockerSwarmServices() (map[string]containerInfo, error) {
	out, err := exec.Command("docker", "stack", "services", "obelisk", "--format", "json").Output()
	if err != nil {
		return nil, err
	}

	type row struct {
		Name     string `json:"Name"`
		Replicas string `json:"Replicas"`
	}

	result := make(map[string]containerInfo)

	parseRows := func(rows []row) {
		for _, r := range rows {
			// Strip stack prefix "obelisk_" to get the module name.
			name := r.Name
			if len(name) > 8 && name[:8] == "obelisk_" {
				name = name[8:]
			}
			result[name] = containerInfo{
				State:    replicasToState(r.Replicas),
				Replicas: r.Replicas,
			}
		}
	}

	// docker stack services outputs a JSON array or NDJSON depending on version.
	var rows []row
	if err := json.Unmarshal(out, &rows); err == nil {
		parseRows(rows)
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
			parseRows([]row{r})
		}
	}
	return result, nil
}

// replicasToState derives a display state from the Swarm "running/desired" string.
func replicasToState(replicas string) string {
	if replicas == "" {
		return "unknown"
	}
	slash := strings.Index(replicas, "/")
	if slash < 0 {
		return replicas
	}
	running := replicas[:slash]
	desired := replicas[slash+1:]
	if desired == "0" {
		return "stopped"
	}
	if running == desired {
		return "running"
	}
	if running == "0" {
		return "starting"
	}
	return "degraded"
}

func sortedModuleKeys(m map[string]*config.Module) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
