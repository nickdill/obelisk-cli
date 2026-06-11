package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Module struct {
	Image     string            `yaml:"image"`
	GitSource string            `yaml:"git_source"`
	Port      int               `yaml:"port"`
	Domain    string            `yaml:"domain"`
	Type      string            `yaml:"type"`
	Env       map[string]string `yaml:"env"`
}

type Config struct {
	Version string             `yaml:"version"`
	Name    string             `yaml:"name"`
	Type    string             `yaml:"type"`
	Port    int                `yaml:"port"`
	Modules map[string]*Module `yaml:"modules"`
}

func IsModule() bool {
	cfg, err := Load()
	if err != nil {
		return false
	}
	return cfg.Type == "module"
}

func Path() string {
	if _, err := os.Stat("obelisk.local.yml"); err == nil {
		return "obelisk.local.yml"
	}
	return "obelisk.yml"
}

func Load() (*Config, error) {
	path := Path()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}
	return &cfg, nil
}

type ModuleConfig struct {
	Version string `yaml:"version"`
	Name    string `yaml:"name"`
	Port    int    `yaml:"port"`
}

func LoadModule() (*ModuleConfig, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	return &ModuleConfig{
		Version: cfg.Version,
		Name:    cfg.Name,
		Port:    cfg.Port,
	}, nil
}
