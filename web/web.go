// Package web assembles the DHCP server as a panel module: configuration,
// reservations and leases in SQLite, the server itself, the JSON API, and
// the browser UI (native ES modules in ui/, no build step).
//
// A DHCP value satisfies the panel module contract without depending on
// any panel: Handler serves the API with relative routes and Assets holds
// the UI with manifest.json at its root. The standalone command and a
// panel build it the same way, through New.
package web

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"net/http"

	"github.com/lsongdev/dhcp-go/api"
	"github.com/lsongdev/dhcp-go/store"
)

//go:embed ui
var embedded embed.FS

// UI is the module's browser assets; manifest.json describes them.
var UI = func() fs.FS {
	sub, err := fs.Sub(embedded, "ui")
	if err != nil {
		panic(err) // the embed directive guarantees the directory exists
	}
	return sub
}()

// Options configures the module; see api.Options.
type Options = api.Options

// DHCP is the assembled module.
type DHCP struct {
	Service *api.Service
	store   *store.SQLite
}

// New migrates the dhcp_ tables in db (shareable with other modules) and
// starts the server if its saved configuration enables it; a new
// installation leaves it disabled. The caller owns db.
func New(ctx context.Context, db *sql.DB, opts Options) (*DHCP, error) {
	st, err := store.New(ctx, db)
	if err != nil {
		return nil, err
	}
	return NewWithStore(ctx, st, opts)
}

// NewWithStore assembles the module on st, which it closes on Close.
func NewWithStore(ctx context.Context, st *store.SQLite, opts Options) (*DHCP, error) {
	service, err := api.NewService(ctx, st, opts)
	if err != nil {
		return nil, err
	}
	return &DHCP{Service: service, store: st}, nil
}

// Handler serves the module's API with relative routes.
func (d *DHCP) Handler() http.Handler { return d.Service.Handler() }

// Assets returns the browser UI; manifest.json is at its root.
func (d *DHCP) Assets() fs.FS { return UI }

// Close stops the server and releases the store; a database passed to New
// stays open.
func (d *DHCP) Close() error {
	d.Service.Close()
	return d.store.Close()
}
