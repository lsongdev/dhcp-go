// Package web assembles the DHCP server as a panel module: configuration,
// reservations and leases in a supplied store, the server itself, the JSON API, and
// the browser UI (native ES modules in ui/, no build step).
//
// A DHCP value satisfies the panel module contract without depending on
// any panel: Handler serves the API with relative routes and Assets holds
// the UI with manifest.json at its root. The standalone command and a
// panel build it the same way, through New.
package web

import (
	"context"
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
}

// New assembles the module on st and starts the server if enabled.
// The caller owns the store's lifetime.
func New(ctx context.Context, st store.Store, opts Options) (*DHCP, error) {
	service, err := api.NewService(ctx, st, opts)
	if err != nil {
		return nil, err
	}
	return &DHCP{Service: service}, nil
}

// Handler serves the module's API with relative routes.
func (d *DHCP) Handler() http.Handler { return d.Service.Handler() }

// Assets returns the browser UI; manifest.json is at its root.
func (d *DHCP) Assets() fs.FS { return UI }

// Close stops the server.
func (d *DHCP) Close() error { return d.Service.Close() }
