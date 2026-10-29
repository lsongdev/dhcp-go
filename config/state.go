package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lsongdev/dhcp-go/server"
	"github.com/lsongdev/dhcp-go/store"
)

type state struct {
	NextID       int64                `yaml:"next_id"`
	Reservations []server.Reservation `yaml:"reservations"`
	Leases       []server.Lease       `yaml:"leases"`
}

func (f *File) loadState() (state, error) {
	var st state
	cfg, err := Load(f.Path)
	if err != nil {
		return st, err
	}
	data, err := os.ReadFile(cfg.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&st); err != nil && !errors.Is(err, io.EOF) {
		return st, fmt.Errorf("%s: %w", cfg.StateFile, err)
	}
	return st, nil
}

func (f *File) updateState(ctx context.Context, change func(*state) error, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	st, err := f.loadState()
	if err != nil {
		return err
	}
	if err := change(&st); err != nil {
		return err
	}
	cfg, err := Load(f.Path)
	if err != nil {
		return err
	}
	values := map[string]any{}
	for _, key := range keys {
		switch key {
		case "next_id":
			values[key] = st.NextID
		case "reservations":
			values[key] = st.Reservations
		case "leases":
			values[key] = st.Leases
		}
	}
	return writeSections(ctx, cfg.StateFile, values)
}

func (f *File) read(ctx context.Context) (state, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return state{}, err
	}
	return f.loadState()
}

func (f *File) Reservations(ctx context.Context) ([]server.Reservation, error) {
	c, err := f.read(ctx)
	return append([]server.Reservation{}, c.Reservations...), err
}

func (f *File) Reservation(ctx context.Context, id int64) (server.Reservation, error) {
	c, err := f.read(ctx)
	if err != nil {
		return server.Reservation{}, err
	}
	for _, r := range c.Reservations {
		if r.ID == id {
			return r, nil
		}
	}
	return server.Reservation{}, store.ErrNotFound
}

func reservationConflict(c *state, r server.Reservation) error {
	for _, old := range c.Reservations {
		if (r.ID == 0 || old.ID != r.ID) && (old.MAC == r.MAC || old.IP == r.IP) {
			return store.ErrConflict
		}
	}
	return nil
}

func (f *File) CreateReservation(ctx context.Context, r *server.Reservation) error {
	saved := *r
	err := f.updateState(ctx, func(c *state) error {
		saved.ID = 0
		if err := reservationConflict(c, saved); err != nil {
			return err
		}
		id := c.NextID
		if id < 1 {
			id = 1
		}
		for _, old := range c.Reservations {
			if old.ID >= id {
				id = old.ID + 1
			}
		}
		now := time.Now().UTC()
		saved.ID, saved.CreatedAt, saved.UpdatedAt = id, now, now
		c.NextID = id + 1
		c.Reservations = append(c.Reservations, saved)
		return nil
	}, "next_id", "reservations")
	if err == nil {
		*r = saved
	}
	return err
}

func (f *File) UpdateReservation(ctx context.Context, r *server.Reservation) error {
	saved := *r
	err := f.updateState(ctx, func(c *state) error {
		for i, old := range c.Reservations {
			if old.ID != saved.ID {
				continue
			}
			if err := reservationConflict(c, saved); err != nil {
				return err
			}
			saved.CreatedAt, saved.UpdatedAt = old.CreatedAt, time.Now().UTC()
			c.Reservations[i] = saved
			return nil
		}
		return store.ErrNotFound
	}, "reservations")
	if err == nil {
		*r = saved
	}
	return err
}

func (f *File) DeleteReservation(ctx context.Context, id int64) error {
	return f.updateState(ctx, func(c *state) error {
		for i, r := range c.Reservations {
			if r.ID == id {
				c.Reservations = append(c.Reservations[:i], c.Reservations[i+1:]...)
				break
			}
		}
		return nil
	}, "reservations")
}

func (f *File) Leases(ctx context.Context) ([]server.Lease, error) {
	c, err := f.read(ctx)
	return append([]server.Lease{}, c.Leases...), err
}

func (f *File) SaveLease(l server.Lease) error {
	return f.updateState(context.Background(), func(c *state) error {
		for i, old := range c.Leases {
			if old.IP == l.IP {
				c.Leases[i] = l
				return nil
			}
		}
		c.Leases = append(c.Leases, l)
		return nil
	}, "leases")
}

func (f *File) DeleteLease(ip string) error {
	return f.updateState(context.Background(), func(c *state) error {
		for i, l := range c.Leases {
			if l.IP == ip {
				c.Leases = append(c.Leases[:i], c.Leases[i+1:]...)
				break
			}
		}
		return nil
	}, "leases")
}

func (f *File) PruneLeases(ctx context.Context, before time.Time) error {
	// Avoid rewriting the state file on every startup when nothing expired.
	c, err := f.read(ctx)
	if err != nil {
		return err
	}
	expired := false
	for _, l := range c.Leases {
		if l.ExpiresAt.Before(before) {
			expired = true
			break
		}
	}
	if !expired {
		return nil
	}
	return f.updateState(ctx, func(c *state) error {
		kept := c.Leases[:0]
		for _, l := range c.Leases {
			if !l.ExpiresAt.Before(before) {
				kept = append(kept, l)
			}
		}
		c.Leases = kept
		return nil
	}, "leases")
}
