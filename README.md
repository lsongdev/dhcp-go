# dhcp-go

> DHCP client and server in Go

- `dhcp4`: the protocol: messages, options, a UDP server and a client.
- `server`, `store`, `api`, `web`, `cmd/dhcp-go`: a DHCP server for one
  network with reservations, persisted leases and a web UI, configured from
  a YAML file, or mounted in a panel such as OhMyServer.

## Example

### DHCP Client

```go
package main

import "github.com/lsongdev/dhcp-go/dhcp4"

func main() {
  c, err := dhcp4.NewClient(&dhcp4.ClientConfig{
    Mac: "aa:bb:cc:dd:ee:ff",
    // Hostname: "mypc",
    // Server:   "192.168.2.1",
    // ClientIP: "192.168.2.185",
  })
  if err != nil {
    log.Fatal(err)
  }
  defer c.Close()
  offer, err := c.Discover()
  if err != nil {
    log.Fatal(err)
  }
  // c.SetServer(offer.ServerIPAddr.String())
  // log.Println(offer)
  ack, err := c.Request(offer)
  // ack, err := c.Decline(offer, "no reason")
  // ack, err := c.Renew()
  // ack, err := c.Inform()
  if err != nil {
    log.Fatal(err)
  }
  log.Println("Received:", ack.MessageType(), ack.String())
}
```

### DHCP Server

A ready-to-run server with a web UI: see [Server](#server) below. To embed
the protocol only, implement `dhcp4.Handler` and serve it on a socket from
`dhcp4.ListenInterface`, which ties it to one interface.

### DHCP Proxy

```go
```

## Server

### Safety

Only one DHCP server should answer on a network: a second one hands out
addresses that conflict with the first, and devices lose their connection.
So:

- A new installation is **off**. Turning it on asks for confirmation and
  says to turn off the router's DHCP server first.
- It serves one interface. Its socket is bound to that interface
  (`SO_BINDTODEVICE` on Linux, `IP_BOUND_IF` on macOS), so it never answers
  on other networks, Docker bridges included. Other systems have no such
  option: there it hears every interface.
- It is authoritative for its network: a client asking to keep an address
  that is not on the network, or is another client's, gets a DHCPNAK and
  starts over.
- A client that finds its address in use (DHCPDECLINE) makes the server
  set the address aside for 10 minutes.

### How addresses are chosen

On DHCPDISCOVER a client is offered, in order: its reservation (and only
that, inside or outside the range); the address offered or leased to it
before; the address it asks for; an address never handed out; then the one
whose lease expired longest ago. Offers are held for a minute. Leases are
saved in SQLite, so clients keep their addresses across restarts; expired
leases are kept for 30 days so returning devices get their old address.
Clients are identified by MAC address.

Replies to a client without an address are broadcast, since unicasting to
it needs an ARP entry a UDP socket cannot write; replies through a relay
agent go to the agent.

### Run

```sh
cp dhcp.example.yaml dhcp.yaml   # then edit it
go build -o dhcp-go ./cmd/dhcp-go
sudo ./dhcp-go -config dhcp.yaml
```

The file sets where the web UI listens (`listen`, with an optional
`password` or `DHCP_GO_PASSWORD`), the SQLite file for reservations and
leases (`database`), and the server's settings (`server`). Changes made in
the UI are written back to the `server` section; the rest of the file,
comments included, is kept.

Serving DHCP binds UDP port 67, which needs root (or
`CAP_NET_BIND_SERVICE`, plus `CAP_NET_RAW` for the interface binding on
Linux). Without a password the UI is served without authentication, which
is only safe on a loopback address.

### As a panel module

The server and its UI follow the panel module contract (see OhMyServer's
`docs/module-contract.md`); OhMyServer mounts it as DHCP:

```go
d, err := web.New(ctx, db, web.Options{}) // tables are prefixed dhcp_; settings too
d.Handler()                               // JSON API, relative routes
d.Assets()                                // UI with manifest.json
```

### API

Responses are `{"data": …}`, errors `{"error": {"code", "message"}}`.

| Method | Path | |
|---|---|---|
| GET | `/api/status` | Whether the server is on and running (or why not), its address, range size and counts |
| GET/PUT | `/api/config` | The configuration; saving it restarts the server |
| GET | `/api/interfaces` | Interfaces with an IPv4 address, to choose from |
| GET | `/api/leases` | Leases, expired ones included (`active`, `reserved`) |
| DELETE | `/api/leases/{ip}` | Forget a lease |
| GET/POST | `/api/reservations` | Reservations: `mac`, `ip`, `hostname`, `description` |
| PUT/DELETE | `/api/reservations/{id}` | |
| GET | `/api/events` | Recent requests and answers, newest first (in memory) |
| GET | `/api/summary` | Counts for an overview |

## Reference

+ https://tools.ietf.org/html/rfc2131
+ https://tools.ietf.org/html/rfc2132
+ https://tools.ietf.org/html/rfc4039
+ https://tools.ietf.org/html/rfc4702

## License

---