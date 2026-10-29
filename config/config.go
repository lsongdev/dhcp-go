// Package config persists DHCP settings and runtime state in separate YAML files.
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
	"github.com/lsongdev/dhcp-go/store"
)

// Config is the file's content.
type Config struct {
	// Listen is the address of the web UI and API; empty serves none.
	Listen string `yaml:"listen"`
	// Password requires HTTP basic authentication on the UI and API (any
	// user name). The DHCP_GO_PASSWORD environment variable overrides it.
	Password string `yaml:"password"`
	// Server holds the DHCP server's settings.
	Server server.Config `yaml:"server"`
	// StateFile stores reservations and leases, relative to this configuration.
	StateFile string `yaml:"state_file"`
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
	if c.StateFile == "" {
		c.StateFile = "dhcp.state.yaml"
	}
	if !filepath.IsAbs(c.StateFile) {
		c.StateFile = filepath.Join(filepath.Dir(path), c.StateFile)
	}
	configPath, err := filepath.Abs(path)
	if err != nil {
		return c, err
	}
	statePath, err := filepath.Abs(c.StateFile)
	if err != nil {
		return c, err
	}
	if configPath == statePath {
		return c, fmt.Errorf("state_file must differ from the configuration file")
	}
	return c, nil
}

// File implements store.Store with settings in Path and runtime data in StateFile.
// Saves preserve unrelated sections and comments. Use one File per service.
type File struct {
	Path string
	mu   sync.Mutex
}

// Config returns the file's server section.
func (f *File) Config(ctx context.Context) (server.Config, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return server.Config{}, false, err
	}
	c, err := Load(f.Path)
	return c.Server, err == nil, err
}

// SaveConfig replaces the file's server section.
func (f *File) SaveConfig(ctx context.Context, cfg server.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := Load(f.Path); err != nil {
		return err
	}
	return writeSections(ctx, f.Path, map[string]any{"server": cfg})
}

// writeSections atomically replaces selected sections, preserving other content.
func writeSections(ctx context.Context, path string, values map[string]any) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
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
		return fmt.Errorf("%s: the top level is not a mapping", path)
	}
	for key, v := range values {
		var value yaml.Node
		if err := value.Encode(v); err != nil {
			return err
		}
		replaced := false
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == key {
				value.HeadComment, value.LineComment = root.Content[i+1].HeadComment, root.Content[i+1].LineComment
				root.Content[i+1] = &value
				replaced = true
			}
		}
		if !replaced {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &value)
		}
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	// Write a sibling file and rename it over, so a crash never leaves a
	// half-written configuration.
	mode := os.FileMode(0o600)
	info, err := os.Stat(path)
	if err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dhcp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

var _ store.Store = (*File)(nil)
