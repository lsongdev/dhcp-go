// Package api runs the DHCP server from its saved configuration and
// manages it, as a JSON API with relative routes.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lsongdev/dhcp-go/dhcp4"
	"github.com/lsongdev/dhcp-go/server"
	"github.com/lsongdev/dhcp-go/store"
)

// ConfigStore keeps the server configuration. store.SQLite is one; the
// standalone command keeps it in its YAML file instead (config.File).
type ConfigStore interface {
	// Config returns the saved configuration; ok is false before the
	// first save.
	Config(ctx context.Context) (cfg server.Config, ok bool, err error)
	SaveConfig(ctx context.Context, cfg server.Config) error
}

// Options configures the service.
type Options struct {
	Logger *slog.Logger
	// Config keeps the configuration; nil keeps it in the SQLite store.
	Config ConfigStore
	// Port is the server's UDP port; 0 means 67. Clients always send to 67:
	// another port is only useful for tests.
	Port int
	// Lookup and Listen replace server.LookupNetwork and
	// dhcp4.ListenInterface, for tests.
	Lookup func(name string) (server.Network, error)
	Listen func(iface string, port int) (*net.UDPConn, error)
}

// retryInterval is how often an enabled server that failed to start (no
// address on its interface yet, port in use) is retried.
const retryInterval = 30 * time.Second

// keepExpired is how long expired leases are kept, so returning clients
// get their old addresses.
const keepExpired = 30 * 24 * time.Hour

// Service runs the server and manages its configuration, reservations and
// leases.
type Service struct {
	store *store.SQLite
	opts  Options
	log   *slog.Logger

	mu     sync.Mutex
	config server.Config
	srv    *server.Server // nil while stopped
	err    error          // why the server is not running, if enabled

	stop chan struct{}
	wg   sync.WaitGroup
}

// NewService loads the configuration and starts the server if it is
// enabled. A server that cannot start is reported in Status and retried,
// not returned as an error: the panel stays up either way.
func NewService(ctx context.Context, st *store.SQLite, opts Options) (*Service, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Port == 0 {
		opts.Port = 67
	}
	if opts.Lookup == nil {
		opts.Lookup = server.LookupNetwork
	}
	if opts.Listen == nil {
		opts.Listen = dhcp4.ListenInterface
	}
	if opts.Config == nil {
		opts.Config = st
	}
	cfg, _, err := opts.Config.Config(ctx)
	if err != nil {
		return nil, err
	}
	if err := st.PruneLeases(ctx, time.Now().Add(-keepExpired)); err != nil {
		return nil, err
	}
	s := &Service{store: st, opts: opts, log: opts.Logger, config: cfg, stop: make(chan struct{})}
	s.mu.Lock()
	s.restart(ctx)
	s.mu.Unlock()
	s.wg.Add(1)
	go s.retry()
	return s, nil
}

// Close stops the server.
func (s *Service) Close() error {
	close(s.stop)
	s.wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdown()
	return nil
}

func (s *Service) retry() {
	defer s.wg.Done()
	t := time.NewTicker(retryInterval)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			if s.config.Enabled && s.srv == nil {
				s.restart(context.Background())
			}
			s.mu.Unlock()
		}
	}
}

// restart stops the server and starts it again if enabled. The caller
// holds s.mu.
func (s *Service) restart(ctx context.Context) {
	s.shutdown()
	s.err = nil
	if !s.config.Enabled {
		return
	}
	if err := s.start(ctx); err != nil {
		s.err = err
		s.log.Error("dhcp: the server did not start", "interface", s.config.Interface, "err", err)
	}
}

// start starts the server. The caller holds s.mu.
func (s *Service) start(ctx context.Context) error {
	cfg := s.config
	n, err := s.opts.Lookup(cfg.Interface)
	if err != nil {
		return err
	}
	// The interface's address may have changed since the configuration
	// was saved.
	if err := cfg.Normalize(func(string) (server.Network, error) { return n, nil }); err != nil {
		return err
	}
	leases, err := s.store.Leases(ctx)
	if err != nil {
		return err
	}
	reservations, err := s.store.Reservations(ctx)
	if err != nil {
		return err
	}
	conn, err := s.opts.Listen(cfg.Interface, s.opts.Port)
	if err != nil {
		return err
	}
	srv := server.New(cfg, n, server.NewPool(cfg, n, leases, reservations, s.store), s.log)
	s.srv = srv
	go srv.Serve(conn)
	go func() {
		<-srv.Done()
		if err := srv.Err(); err != nil {
			s.mu.Lock()
			if s.srv == srv {
				s.srv, s.err = nil, err
			}
			s.mu.Unlock()
		}
	}()
	s.log.Info("dhcp: serving", "interface", n.Interface, "address", n.IP, "range", cfg.RangeStart+"-"+cfg.RangeEnd)
	return nil
}

// shutdown stops a running server. The caller holds s.mu.
func (s *Service) shutdown() {
	if s.srv == nil {
		return
	}
	s.srv.Close()
	<-s.srv.Done()
	s.srv = nil
}

// Status describes the server.
type Status struct {
	Enabled      bool   `json:"enabled"`
	Running      bool   `json:"running"`
	Error        string `json:"error,omitempty"` // why an enabled server is not running
	Interface    string `json:"interface"`
	ServerIP     string `json:"server_ip,omitempty"`
	Subnet       string `json:"subnet,omitempty"`
	RangeStart   string `json:"range_start"`
	RangeEnd     string `json:"range_end"`
	PoolSize     int    `json:"pool_size"`
	Active       int    `json:"active_leases"`
	Reservations int    `json:"reservations"`
	Requests     uint64 `json:"requests"`
}

// Status reports whether the server runs, and its counts.
func (s *Service) Status(ctx context.Context) (Status, error) {
	reservations, err := s.store.Reservations(ctx)
	if err != nil {
		return Status{}, err
	}
	leases, err := s.Leases(ctx)
	if err != nil {
		return Status{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{
		Enabled: s.config.Enabled, Running: s.srv != nil, Interface: s.config.Interface,
		RangeStart: s.config.RangeStart, RangeEnd: s.config.RangeEnd, Reservations: len(reservations),
	}
	if s.err != nil {
		st.Error = s.err.Error()
	}
	if s.srv != nil {
		n := s.srv.Network()
		st.ServerIP, st.Subnet = n.IP.String(), n.Subnet.String()
		st.PoolSize, st.Requests = s.srv.Pool.Size(), s.srv.Requests()
	}
	for _, l := range leases {
		if l.Active {
			st.Active++
		}
	}
	return st, nil
}

// Config returns the configuration.
func (s *Service) Config() server.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.config
	cfg.DNS = append([]string{}, cfg.DNS...)
	return cfg
}

// SetConfig checks, saves and applies a configuration, restarting the
// server.
func (s *Service) SetConfig(ctx context.Context, cfg server.Config) (Status, error) {
	if err := cfg.Normalize(s.opts.Lookup); err != nil {
		return Status{}, err
	}
	if err := s.opts.Config.SaveConfig(ctx, cfg); err != nil {
		return Status{}, err
	}
	s.mu.Lock()
	s.config = cfg
	s.restart(ctx)
	s.mu.Unlock()
	return s.Status(ctx)
}

// Interface is a network interface the server could serve.
type Interface struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac,omitempty"`
	Up        bool     `json:"up"`
	Addresses []string `json:"addresses"` // IPv4, CIDR notation
}

// Interfaces lists the interfaces with an IPv4 address, loopback excluded.
func (s *Service) Interfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	list := []Interface{}
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		var v4 []string
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				v4 = append(v4, n.String())
			}
		}
		if len(v4) == 0 {
			continue
		}
		list = append(list, Interface{Name: i.Name, MAC: i.HardwareAddr.String(), Up: i.Flags&net.FlagUp != 0, Addresses: v4})
	}
	return list, nil
}

// LeaseView is a lease as listed.
type LeaseView struct {
	server.Lease
	Active   bool `json:"active"`   // not expired
	Reserved bool `json:"reserved"` // the client has a reservation
}

// Leases lists the leases: from the running server, or as stored.
func (s *Service) Leases(ctx context.Context) ([]LeaseView, error) {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	var leases []server.Lease
	if srv != nil {
		leases = srv.Pool.Leases()
	} else {
		var err error
		if leases, err = s.store.Leases(ctx); err != nil {
			return nil, err
		}
	}
	reservations, err := s.store.Reservations(ctx)
	if err != nil {
		return nil, err
	}
	reserved := map[string]bool{}
	for _, r := range reservations {
		reserved[r.MAC] = true
	}
	now := time.Now()
	list := make([]LeaseView, len(leases))
	for i, l := range leases {
		list[i] = LeaseView{Lease: l, Active: now.Before(l.ExpiresAt), Reserved: reserved[l.MAC]}
	}
	sort.SliceStable(list, func(i, j int) bool { return ipLess(list[i].IP, list[j].IP) })
	return list, nil
}

func ipLess(a, b string) bool {
	x, y := net.ParseIP(a).To4(), net.ParseIP(b).To4()
	if x == nil || y == nil {
		return a < b
	}
	return string(x) < string(y)
}

// RemoveLease forgets a lease.
func (s *Service) RemoveLease(ip string) error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv != nil {
		return srv.Pool.Remove(ip)
	}
	return s.store.DeleteLease(ip)
}

// Events returns the running server's recent exchanges, newest first.
func (s *Service) Events() []server.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return []server.Event{}
	}
	return s.srv.Events()
}

// Reservations lists the reservations.
func (s *Service) Reservations(ctx context.Context) ([]server.Reservation, error) {
	return s.store.Reservations(ctx)
}

// CreateReservation saves a reservation and applies it.
func (s *Service) CreateReservation(ctx context.Context, r server.Reservation) (server.Reservation, error) {
	if err := s.checkReservation(&r); err != nil {
		return r, err
	}
	if err := s.store.CreateReservation(ctx, &r); err != nil {
		return r, err
	}
	return r, s.applyReservations(ctx)
}

// UpdateReservation changes a reservation and applies it.
func (s *Service) UpdateReservation(ctx context.Context, id int64, r server.Reservation) (server.Reservation, error) {
	old, err := s.store.Reservation(ctx, id)
	if err != nil {
		return r, err
	}
	r.ID, r.CreatedAt = id, old.CreatedAt
	if err := s.checkReservation(&r); err != nil {
		return r, err
	}
	if err := s.store.UpdateReservation(ctx, &r); err != nil {
		return r, err
	}
	return r, s.applyReservations(ctx)
}

// DeleteReservation removes a reservation.
func (s *Service) DeleteReservation(ctx context.Context, id int64) error {
	if err := s.store.DeleteReservation(ctx, id); err != nil {
		return err
	}
	return s.applyReservations(ctx)
}

// checkReservation normalizes r and, when the server's network is known,
// checks that r's address is a host address on it.
func (s *Service) checkReservation(r *server.Reservation) error {
	if err := r.Normalize(); err != nil {
		return err
	}
	iface := s.Config().Interface
	if iface == "" {
		return nil
	}
	n, err := s.opts.Lookup(iface)
	if err != nil {
		return nil // checked when the server starts
	}
	ip := net.ParseIP(r.IP)
	switch {
	case !n.Subnet.Contains(ip):
		return server.Invalid("%s is not on %s, the network of %s", r.IP, n.Subnet, iface)
	case ip.Equal(n.IP):
		return server.Invalid("%s is this server's address", r.IP)
	case ip.Equal(n.Subnet.IP) || ip.Equal(n.Broadcast()):
		return server.Invalid("%s is the network or broadcast address", r.IP)
	}
	return nil
}

func (s *Service) applyReservations(ctx context.Context) error {
	list, err := s.store.Reservations(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		s.srv.Pool.SetReservations(list)
	}
	return nil
}

// Summary counts leases for an overview.
type Summary struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	Active  int  `json:"active_leases"`
}

// Summary returns the counts.
func (s *Service) Summary(ctx context.Context) (Summary, error) {
	st, err := s.Status(ctx)
	return Summary{Enabled: st.Enabled, Running: st.Running, Active: st.Active}, err
}

var errNoIP = errors.New("invalid IP address")

func parseIP(s string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(s)).To4()
	if ip == nil {
		return "", errNoIP
	}
	return ip.String(), nil
}
