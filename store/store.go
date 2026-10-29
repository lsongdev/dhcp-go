// Package store defines persistence for DHCP configuration, reservations and leases.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/lsongdev/dhcp-go/server"
)

var (
	ErrNotFound = errors.New("reservation not found")
	ErrConflict = errors.New("this MAC address or IP address is already reserved")
)

// Store persists the service state. Callers own its lifetime.
// Implementations must be safe for concurrent use.
type Store interface {
	server.LeaseStore
	Config(context.Context) (server.Config, bool, error)
	SaveConfig(context.Context, server.Config) error
	Reservations(context.Context) ([]server.Reservation, error)
	Reservation(context.Context, int64) (server.Reservation, error)
	CreateReservation(context.Context, *server.Reservation) error
	UpdateReservation(context.Context, *server.Reservation) error
	DeleteReservation(context.Context, int64) error
	Leases(context.Context) ([]server.Lease, error)
	PruneLeases(context.Context, time.Time) error
}
