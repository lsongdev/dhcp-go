package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lsongdev/dhcp-go/server"
)

func TestLoadAndSaveKeepTheRestOfTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dhcp.yaml")
	os.WriteFile(path, []byte(`# my settings
listen: 127.0.0.1:8067 # the UI
server:
  enabled: false
  interface: eth0
`), 0o640)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "127.0.0.1:8067" || c.Server.Interface != "eth0" || c.Database != filepath.Join(filepath.Dir(path), "dhcp.db") {
		t.Fatalf("loaded %+v", c)
	}
	f := &File{Path: path}
	want := server.Config{Enabled: true, Interface: "en0", RangeStart: "10.0.0.100", RangeEnd: "10.0.0.200", DNS: []string{"10.0.0.2"}}
	if err := f.SaveConfig(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := f.Config(context.Background())
	if err != nil || !ok || got.Interface != "en0" || got.RangeEnd != "10.0.0.200" || len(got.DNS) != 1 || !got.Enabled {
		t.Fatalf("saved %+v %v %v", got, ok, err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# my settings") || !strings.Contains(string(data), "# the UI") {
		t.Fatalf("comments lost:\n%s", data)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v", info.Mode())
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dhcp.yaml")
	os.WriteFile(path, []byte("lisen: :8067\n"), 0o600)
	if _, err := Load(path); err == nil {
		t.Fatal("a misspelt key was accepted")
	}
}
