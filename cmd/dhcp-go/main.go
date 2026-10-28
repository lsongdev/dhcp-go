// Command dhcp-go runs a DHCP server from a YAML configuration file, with
// a web UI for its settings, reservations and leases. A panel shell can
// mount the same module (web.New) beside others instead.
//
//	dhcp-go -config dhcp.yaml   serve
//	dhcp-go client              run the example client (examples/client.go)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lsongdev/dhcp-go/config"
	"github.com/lsongdev/dhcp-go/examples"
	"github.com/lsongdev/dhcp-go/store"
	"github.com/lsongdev/dhcp-go/web"
)

const usage = `usage: dhcp-go [flags]
       dhcp-go client

Runs the DHCP server configured in the YAML file (see dhcp.example.yaml)
and serves its web UI. Settings changed in the UI are written back to the
file. Serving DHCP binds UDP port 67, so it usually runs as root.

Flags:
`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "client" {
		examples.RunClient()
		return
	}
	flag.Usage = func() { fmt.Fprint(flag.CommandLine.Output(), usage); flag.PrintDefaults() }
	path := flag.String("config", "dhcp.yaml", "configuration file")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	if err := run(*path, logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(path string, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	module, err := web.NewWithStore(ctx, st, web.Options{Logger: logger, Config: &config.File{Path: path}})
	if err != nil {
		st.Close()
		return err
	}
	defer module.Close()
	if cfg.Listen == "" {
		logger.Info("dhcp-go running without a web UI", "config", path)
		<-ctx.Done()
		return nil
	}

	password := cfg.Password
	if env := os.Getenv("DHCP_GO_PASSWORD"); env != "" {
		password = env
	}
	if password == "" && !isLoopback(cfg.Listen) {
		logger.Warn("serving the UI without authentication on a non-loopback address; set a password")
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: module.Standalone(password), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	logger.Info("dhcp-go listening", "url", "http://"+cfg.Listen, "config", path, "db", cfg.Database)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func isLoopback(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}
