package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nickdill/obelisk/internal/identity"
)

type Server struct {
	Name     string
	URL      string
	LastSeen time.Time
}

type serverEntry struct {
	URL      string    `yaml:"url"`
	LastSeen time.Time `yaml:"last_seen,omitempty"`
}

type registryFile struct {
	Servers map[string]serverEntry `yaml:"servers"`
}

func registryPath() (string, error) {
	dir, err := identity.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "servers.yml"), nil
}

func load() (registryFile, error) {
	path, err := registryPath()
	if err != nil {
		return registryFile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return registryFile{Servers: map[string]serverEntry{}}, nil
		}
		return registryFile{}, err
	}
	var r registryFile
	if err := yaml.Unmarshal(data, &r); err != nil {
		return registryFile{}, err
	}
	if r.Servers == nil {
		r.Servers = map[string]serverEntry{}
	}
	return r, nil
}

func save(r registryFile) error {
	path, err := registryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Add registers a server. Overwrites if the name already exists.
func Add(name, url string) error {
	r, err := load()
	if err != nil {
		return err
	}
	r.Servers[name] = serverEntry{URL: url, LastSeen: time.Now().UTC()}
	return save(r)
}

// Touch updates the last_seen timestamp for a server.
func Touch(name string) error {
	r, err := load()
	if err != nil {
		return err
	}
	e, ok := r.Servers[name]
	if !ok {
		return fmt.Errorf("server %q not found", name)
	}
	e.LastSeen = time.Now().UTC()
	r.Servers[name] = e
	return save(r)
}

// Remove deletes a server from the registry.
func Remove(name string) error {
	r, err := load()
	if err != nil {
		return err
	}
	if _, ok := r.Servers[name]; !ok {
		return fmt.Errorf("server %q not found", name)
	}
	delete(r.Servers, name)
	return save(r)
}

// List returns all registered servers sorted by name.
func List() ([]Server, error) {
	r, err := load()
	if err != nil {
		return nil, err
	}
	servers := make([]Server, 0, len(r.Servers))
	for name, e := range r.Servers {
		servers = append(servers, Server{Name: name, URL: e.URL, LastSeen: e.LastSeen})
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	return servers, nil
}

// Resolve finds a server by name. If name is empty and exactly one server is
// registered, it returns that server. Returns an error if ambiguous or missing.
func Resolve(name string) (Server, error) {
	r, err := load()
	if err != nil {
		return Server{}, err
	}
	if name != "" {
		e, ok := r.Servers[name]
		if !ok {
			return Server{}, fmt.Errorf("server %q not found; run `obelisk server list`", name)
		}
		return Server{Name: name, URL: e.URL, LastSeen: e.LastSeen}, nil
	}
	if len(r.Servers) == 0 {
		return Server{}, errors.New("no servers registered; run `obelisk server add <name> <url>`")
	}
	if len(r.Servers) > 1 {
		return Server{}, errors.New("multiple servers registered; specify one with --server <name>")
	}
	for name, e := range r.Servers {
		return Server{Name: name, URL: e.URL, LastSeen: e.LastSeen}, nil
	}
	return Server{}, errors.New("unreachable")
}
