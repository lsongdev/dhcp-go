// Package config reads the standalone command's YAML file: where the web UI
// listens, where reservations and leases are stored, and the DHCP server's
// settings, which the UI writes back to the file when they change.
package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/lsongdev/dhcp-go/server"
)

// Config is the file's content.
type Config struct {
	// Listen is the address of the web UI and API; empty serves none.
	Listen string `yaml:"listen"`
	// Password requires HTTP basic authentication on the UI and API (any
	// user name). The DHCP_GO_PASSWORD environment variable overrides it.
	Password string `yaml:"password"`
	// Database is the SQLite file for reservations and leases, relative to
	// the configuration file.
	Database string `yaml:"database"`
	// Server holds the DHCP server's settings.
	Server server.Config `yaml:"server"`
}

// Load reads the file at path and fills in defaults.
func Load(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	if c.Database == "" {
		c.Database = "dhcp.db"
	}
	if !filepath.IsAbs(c.Database) {
		c.Database = filepath.Join(filepath.Dir(path), c.Database)
	}
	return c, nil
}

// File keeps the server settings in the configuration file. It implements
// api.ConfigStore: saving rewrites only the file's server section, keeping
// the rest, comments included.
type File struct {
	Path string
	mu   sync.Mutex
}

// Config returns the file's server section.
func (f *File) Config(context.Context) (server.Config, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := Load(f.Path)
	return c.Server, err == nil, err
}

// SaveConfig replaces the file's server section.
func (f *File) SaveConfig(_ context.Context, cfg server.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind == 0 { // an empty file
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: the top level is not a mapping", f.Path)
	}
	var value yaml.Node
	if err := value.Encode(cfg); err != nil {
		return err
	}
	replaced := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "server" {
			value.HeadComment, value.LineComment = root.Content[i+1].HeadComment, root.Content[i+1].LineComment
			root.Content[i+1] = &value
			replaced = true
		}
	}
	if !replaced {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "server"}, &value)
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	// Write a sibling file and rename it over, so a crash never leaves a
	// half-written configuration.
	info, err := os.Stat(f.Path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.Path), ".dhcp-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.Path)
}
