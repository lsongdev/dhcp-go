package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/lsongdev/dhcp-go/dhcp4"
	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

// Event is one exchange with a client, for the activity log.
type Event struct {
	Time     time.Time `json:"time"`
	Type     string    `json:"type"`   // the client's message: discover, request, decline, release, inform
	Result   string    `json:"result"` // offer, ack, nak, ignored, or empty when nothing is sent
	MAC      string    `json:"mac"`
	IP       string    `json:"ip,omitempty"`
	Hostname string    `json:"hostname,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

// keepEvents is how many recent events Server keeps.
const keepEvents = 300

// Server answers DHCP requests on one network from a Pool.
type Server struct {
	Pool    *Pool
	config  Config
	network Network
	log     *slog.Logger
	udp     *dhcp4.Server

	mu     sync.Mutex
	events []Event
	done   chan struct{}
	err    error // why Serve stopped, if it failed
}

// New returns a server for cfg on n; Serve starts it.
func New(cfg Config, n Network, pool *Pool, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{Pool: pool, config: cfg, network: n, log: log, done: make(chan struct{})}
	s.udp = dhcp4.NewServer("", s)
	return s
}

// Network returns the network the server answers on.
func (s *Server) Network() Network { return s.network }

// Serve answers requests on conn (see Listen) until Close.
func (s *Server) Serve(conn *net.UDPConn) {
	err := s.udp.Serve(conn)
	if errors.Is(err, dhcp4.ErrServerClosed) {
		err = nil
	} else if err != nil {
		s.log.Error("dhcp: server stopped", "err", err)
	}
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
	close(s.done)
}

// Close stops the server.
func (s *Server) Close() error { return s.udp.Close() }

// Done is closed when Serve returns; Err then says why.
func (s *Server) Done() <-chan struct{} { return s.done }

// Err returns the error that stopped Serve, if any.
func (s *Server) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Requests returns how many requests the server received.
func (s *Server) Requests() uint64 { return s.udp.RequestCount() }

// Events returns recent exchanges, newest first.
func (s *Server) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]Event, len(s.events))
	for i, e := range s.events {
		list[len(s.events)-1-i] = e
	}
	return list
}

func (s *Server) record(e Event) {
	e.Time = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	if len(s.events) > keepEvents {
		s.events = s.events[len(s.events)-keepEvents:]
	}
}

// replyOptions are the options every OFFER and ACK carries.
func (s *Server) replyOptions(lease bool) []options.Option {
	opts := []options.Option{
		options.NewServerIdentifierOption(s.network.IP.String()),
		options.NewSubnetMaskOption(net.IP(s.network.Subnet.Mask).String()),
		options.NewBroadcastAddressOption(s.network.Broadcast().String()),
	}
	if lease {
		secs := uint32(s.config.Lease() / time.Second)
		opts = append(opts,
			options.NewLeaseTimeOption(secs),
			options.NewRenewalTimeOption(secs/2),     // T1, RFC 2131 4.4.5
			options.NewRebindingTimeOption(secs/8*7), // T2
		)
	}
	if s.config.Router != "" {
		opts = append(opts, options.NewRouterOption([]string{s.config.Router}))
	}
	if len(s.config.DNS) > 0 {
		opts = append(opts, options.NewDomainNameServerOption(s.config.DNS))
	}
	if s.config.Domain != "" {
		opts = append(opts, options.NewDomainNameOption(s.config.Domain))
	}
	return opts
}

// ServeDHCP implements dhcp4.Handler, following RFC 2131 section 4.3.
func (s *Server) ServeDHCP(req *dhcp4.Message, rw dhcp4.ResponseWriter) {
	if req.HardwareType != 1 || len(req.ClientHardwareAddr) != 6 {
		return // Ethernet clients only
	}
	if isSet(req.GatewayIPAddr) && !s.network.Subnet.Contains(req.GatewayIPAddr) {
		return // relayed from a network this server does not serve
	}
	mac := req.ClientHardwareAddr.String()
	hostname := req.GetHostName()
	requested := parseIPv4(req.GetRequestedIP())
	serverID := serverIdentifier(req)
	ours := serverID == nil || serverID.Equal(s.network.IP)
	e := Event{MAC: mac, Hostname: hostname}

	switch req.MessageType() {
	case options.DHCPDISCOVER:
		e.Type = "discover"
		ip, err := s.Pool.Offer(mac, requested)
		if err != nil {
			e.Detail = err.Error()
			s.log.Warn("dhcp: no address to offer", "mac", mac, "err", err)
			break
		}
		e.Result, e.IP = "offer", ip.String()
		s.send(rw, dhcp4.NewOfferMessage(req, ip.String()), s.replyOptions(true), &e)

	case options.DHCPREQUEST:
		e.Type = "request"
		var ip net.IP
		switch {
		case serverID != nil: // SELECTING: the client answers an offer
			if !ours {
				s.Pool.Cancel(mac)
				e.Result, e.Detail = "ignored", "chose server "+serverID.String()
				break
			}
			ip = requested
			if ip == nil {
				ip = req.ClientIPAddr
			}
		case requested != nil: // INIT-REBOOT: the client checks its address
			ip = requested
		case isSet(req.ClientIPAddr): // RENEWING or REBINDING
			ip = req.ClientIPAddr
		}
		if e.Result == "ignored" {
			break
		}
		if ip == nil {
			e.Result, e.Detail = "ignored", "no address requested"
			break
		}
		e.IP = ip.String()
		if !s.network.Subnet.Contains(ip) {
			s.nak(rw, req, &e, "address is not on this network")
			break
		}
		lease, err := s.Pool.Ack(mac, ip, hostname)
		if errors.Is(err, ErrUnavailable) {
			s.nak(rw, req, &e, "address not available")
			break
		}
		if err != nil {
			// Bound in memory, not persisted: still answer the client.
			s.log.Error("dhcp: saving a lease failed", "ip", lease.IP, "err", err)
		}
		e.Result = "ack"
		s.send(rw, dhcp4.NewAckMessage(req, ip.String()), s.replyOptions(true), &e)

	case options.DHCPDECLINE:
		e.Type = "decline"
		if ours && requested != nil {
			e.IP = requested.String()
			s.log.Warn("dhcp: a client found its address in use", "ip", requested, "mac", mac)
			if err := s.Pool.Decline(mac, requested); err != nil {
				s.log.Error("dhcp: deleting a lease failed", "ip", requested, "err", err)
			}
		}

	case options.DHCPRELEASE:
		e.Type, e.IP = "release", req.ClientIPAddr.String()
		if ours {
			if err := s.Pool.Release(mac, req.ClientIPAddr); err != nil {
				s.log.Error("dhcp: saving a lease failed", "ip", e.IP, "err", err)
			}
		}

	case options.DHCPINFORM: // configured by hand, asking for the other options
		e.Type, e.IP = "inform", req.ClientIPAddr.String()
		ack := dhcp4.NewAckMessage(req, "0.0.0.0")
		delete(ack.Options, options.OptionCodeLeaseTime)
		ack.ClientIPAddr = req.ClientIPAddr
		e.Result = "ack"
		s.send(rw, ack, s.replyOptions(false), &e)

	default:
		return
	}
	s.record(e)
}

func (s *Server) nak(rw dhcp4.ResponseWriter, req *dhcp4.Message, e *Event, reason string) {
	e.Result, e.Detail = "nak", reason
	s.send(rw, dhcp4.NewNakMessage(req, reason), []options.Option{options.NewServerIdentifierOption(s.network.IP.String())}, e)
}

func (s *Server) send(rw dhcp4.ResponseWriter, m *dhcp4.Message, opts []options.Option, e *Event) {
	if err := rw.WriteResponse(m, opts...); err != nil {
		e.Detail = fmt.Sprintf("sending failed: %v", err)
		s.log.Warn("dhcp: sending a reply failed", "mac", e.MAC, "err", err)
	}
}

func serverIdentifier(m *dhcp4.Message) net.IP {
	o, ok := m.GetOption(options.OptionCodeServerIdentifier).(options.ServerIdentifierOption)
	if !ok {
		return nil
	}
	return o.ServerIdentifier.To4()
}

func isSet(ip net.IP) bool { return ip != nil && !ip.Equal(net.IPv4zero) }
