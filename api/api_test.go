package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/lsongdev/dhcp-go/dhcp4"
	"github.com/lsongdev/dhcp-go/server"
	"github.com/lsongdev/dhcp-go/store"
)

// testService runs the service against a fake eth0 on 192.0.2.0/24 whose
// "port 67" is a loopback socket, so tests never touch a real network.
func testService(t *testing.T) (*Service, *store.SQLite) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "dhcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	_, subnet, _ := net.ParseCIDR("192.0.2.0/24")
	s, err := NewService(context.Background(), st, Options{
		Lookup: func(name string) (server.Network, error) {
			if name != "eth0" {
				return server.Network{}, server.Invalid("no such interface")
			}
			return server.Network{Interface: name, IP: net.ParseIP("192.0.2.1").To4(), Subnet: subnet}, nil
		},
		Listen: func(string, int) (*net.UDPConn, error) {
			return net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, st
}

func call(t *testing.T, h http.Handler, method, path string, body any) (int, map[string]json.RawMessage) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, &buf))
	var out map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestNewInstallationIsOff(t *testing.T) {
	s, _ := testService(t)
	st, err := s.Status(context.Background())
	if err != nil || st.Enabled || st.Running {
		t.Fatalf("status = %+v, %v", st, err)
	}
}

func TestEnableServesAndDisableStops(t *testing.T) {
	s, st := testService(t)
	h := s.Handler()
	cfg := server.Config{Enabled: true, Interface: "eth0", RangeStart: "192.0.2.100", RangeEnd: "192.0.2.199", Router: "192.0.2.1"}
	if code, out := call(t, h, "PUT", "/config", cfg); code != 200 {
		t.Fatalf("PUT /config: %d %s", code, out["error"])
	}
	status, _ := s.Status(context.Background())
	if !status.Running || status.ServerIP != "192.0.2.1" || status.PoolSize != 100 {
		t.Fatalf("status = %+v", status)
	}
	saved, ok, err := st.Config(context.Background())
	if err != nil || !ok || !saved.Enabled {
		t.Fatalf("saved config = %+v %v %v", saved, ok, err)
	}
	cfg.Enabled = false
	call(t, h, "PUT", "/config", cfg)
	if status, _ := s.Status(context.Background()); status.Running {
		t.Fatalf("still running: %+v", status)
	}
}

func TestConfigErrors(t *testing.T) {
	s, _ := testService(t)
	h := s.Handler()
	for name, cfg := range map[string]server.Config{
		"unknown interface": {Enabled: true, Interface: "wlan9", RangeStart: "192.0.2.100", RangeEnd: "192.0.2.199"},
		"range elsewhere":   {Enabled: true, Interface: "eth0", RangeStart: "10.0.0.100", RangeEnd: "10.0.0.199"},
	} {
		if code, _ := call(t, h, "PUT", "/config", cfg); code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", name, code)
		}
	}
	if code, _ := call(t, h, "PUT", "/config", map[string]any{"enabled": true, "bogus": 1}); code != http.StatusBadRequest {
		t.Errorf("unknown field: %d", code)
	}
	if status, _ := s.Status(context.Background()); status.Enabled {
		t.Fatal("a rejected configuration was applied")
	}
}

func TestReservations(t *testing.T) {
	s, _ := testService(t)
	h := s.Handler()
	call(t, h, "PUT", "/config", server.Config{Interface: "eth0", RangeStart: "192.0.2.100", RangeEnd: "192.0.2.199"})
	code, out := call(t, h, "POST", "/reservations", map[string]string{"mac": "00-00-5E-00-53-0A", "ip": "192.0.2.20", "hostname": "printer"})
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, out["error"])
	}
	var r server.Reservation
	json.Unmarshal(out["data"], &r)
	if r.MAC != "00:00:5e:00:53:0a" {
		t.Fatalf("mac = %q", r.MAC)
	}
	for name, body := range map[string]map[string]string{
		"same MAC":       {"mac": "00:00:5e:00:53:0a", "ip": "192.0.2.21"},
		"same IP":        {"mac": "00:00:5e:00:53:0b", "ip": "192.0.2.20"},
		"other network":  {"mac": "00:00:5e:00:53:0c", "ip": "10.0.0.5"},
		"server address": {"mac": "00:00:5e:00:53:0c", "ip": "192.0.2.1"},
		"bad MAC":        {"mac": "nope", "ip": "192.0.2.30"},
	} {
		if code, _ := call(t, h, "POST", "/reservations", body); code != http.StatusConflict && code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", name, code)
		}
	}
	if code, _ := call(t, h, "DELETE", "/reservations/1", nil); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
}

// A request reaches the running server through the socket it listens on.
func TestServerReceivesRequests(t *testing.T) {
	s, _ := testService(t)
	var addr *net.UDPAddr
	s.opts.Listen = func(string, int) (*net.UDPConn, error) {
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err == nil {
			addr = conn.LocalAddr().(*net.UDPAddr)
		}
		return conn, err
	}
	if _, err := s.SetConfig(context.Background(), server.Config{Enabled: true, Interface: "eth0", RangeStart: "192.0.2.100", RangeEnd: "192.0.2.199"}); err != nil {
		t.Fatal(err)
	}
	client, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	m := dhcp4.NewDiscoverMessage()
	m.SetMacAddress("00:00:5e:00:53:0a")
	client.Write(m.Bytes())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if events := s.Events(); len(events) == 1 && events[0].Result == "offer" && events[0].IP == "192.0.2.100" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("events = %+v", s.Events())
}
