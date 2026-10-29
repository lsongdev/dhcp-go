package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lsongdev/dhcp-go/store"

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
	if c.Listen != "127.0.0.1:8067" || c.Server.Interface != "eth0" {
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

func TestStatePersistsSeparately(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "dhcp.yaml")
	original := []byte("# settings\nstate_file: runtime/state.yaml\nserver: {}\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	f := &File{Path: path}
	if leases, err := f.Leases(ctx); err != nil || len(leases) != 0 {
		t.Fatalf("new state: %v %v", leases, err)
	}
	r := server.Reservation{MAC: "00:00:5e:00:53:01", IP: "192.0.2.10"}
	if err := f.CreateReservation(ctx, &r); err != nil {
		t.Fatal(err)
	}
	duplicate := server.Reservation{MAC: r.MAC, IP: "192.0.2.11"}
	if err := f.CreateReservation(ctx, &duplicate); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	now := time.Now().UTC()
	old := server.Lease{IP: "192.0.2.11", MAC: r.MAC, ExpiresAt: now.Add(-time.Hour), UpdatedAt: now}
	active := server.Lease{IP: "192.0.2.12", MAC: r.MAC, ExpiresAt: now.Add(time.Hour), UpdatedAt: now}
	for _, l := range []server.Lease{old, active} {
		if err := f.SaveLease(l); err != nil {
			t.Fatal(err)
		}
	}
	reopened := &File{Path: path}
	got, err := reopened.Reservation(ctx, r.ID)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("reservation after reopen: %+v %v", got, err)
	}
	if err := reopened.PruneLeases(ctx, now); err != nil {
		t.Fatal(err)
	}
	leases, err := reopened.Leases(ctx)
	if err != nil || !reflect.DeepEqual(leases, []server.Lease{active}) {
		t.Fatalf("leases after reopen/prune: %+v %v", leases, err)
	}
	if err := reopened.DeleteReservation(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	next := server.Reservation{MAC: r.MAC, IP: r.IP}
	if err := reopened.CreateReservation(ctx, &next); err != nil || next.ID <= r.ID {
		t.Fatalf("next ID: %+v %v", next, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("DHCP activity changed config: %s %v", data, err)
	}
	statePath := filepath.Join(filepath.Dir(path), "runtime/state.yaml")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.SaveConfig(ctx, server.Config{Interface: "eth0"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(statePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("saving config changed runtime state")
	}
	if info, err := os.Stat(statePath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions: %v %v", info, err)
	}
}

func TestConcurrentStateWritesAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dhcp.yaml")
	if err := os.WriteFile(path, []byte("server: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &File{Path: path}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := f.SaveLease(server.Lease{IP: fmt.Sprintf("192.0.2.%d", i+1)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	leases, err := f.Leases(context.Background())
	if err != nil || len(leases) != 20 {
		t.Fatalf("lost concurrent writes: %d %v", len(leases), err)
	}
	statePath := filepath.Join(filepath.Dir(path), "dhcp.state.yaml")
	broken := []byte("leases: [invalid\n")
	if err := os.WriteFile(statePath, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveLease(server.Lease{IP: "192.0.2.100"}); err == nil {
		t.Fatal("corrupt state was overwritten")
	}
	data, _ := os.ReadFile(statePath)
	if !bytes.Equal(data, broken) {
		t.Fatal("corrupt state changed")
	}
}

func TestStateFileMustDifferFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dhcp.yaml")
	if err := os.WriteFile(path, []byte("state_file: dhcp.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("accepted state_file pointing at config")
	}
}
