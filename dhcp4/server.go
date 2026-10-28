package dhcp4

import (
	"errors"
	"fmt"
	"log"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

var ErrServerClosed = errors.New("dhcp4: server closed")

type IGetRequestedIP interface {
	GetRequestedIP() string
	GetMacAddress() string
	GetLeaseTime() uint32
	GetHostName() string
}

type IGetClientIP interface {
	GetClientIP() string
	GetMacAddress() string
}

type OfferWriter interface {
	SendOffer(ip string, options ...options.Option)
}

type AckWriter interface {
	SendAck(ip string, options ...options.Option)
	SendNak(reason string, options ...options.Option)
}

type ResponseWriter interface {
	OfferWriter
	AckWriter

	WriteResponse(resp *Message, options ...options.Option) error
}

type Handler interface {
	ServeDHCP(req *Message, rw ResponseWriter)
}

// Server is an embeddable DHCPv4 UDP server.
type Server struct {
	Addr       string
	ClientPort int
	Handler    Handler

	mu       sync.RWMutex
	conn     *net.UDPConn
	closed   bool
	requests atomic.Uint64
}

func NewServer(addr string, handler Handler) *Server {
	return &Server{Addr: addr, ClientPort: 68, Handler: handler}
}

func (s *Server) ListenAndServe() error {
	laddr, err := net.ResolveUDPAddr("udp4", s.Addr)
	if err != nil {
		return fmt.Errorf("dhcp4: resolve address %s: %w", s.Addr, err)
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return fmt.Errorf("dhcp4: listen udp %s: %w", s.Addr, err)
	}
	return s.Serve(conn)
}

func (s *Server) Serve(conn *net.UDPConn) error {
	if conn == nil {
		return errors.New("dhcp4: nil UDP connection")
	}
	if s.Handler == nil {
		_ = conn.Close()
		return errors.New("dhcp4: nil handler")
	}
	if err := enableBroadcast(conn); err != nil {
		_ = conn.Close()
		return fmt.Errorf("dhcp4: enable UDP broadcast: %w", err)
	}
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		_ = conn.Close()
		return errors.New("dhcp4: server already serving")
	}
	s.conn = conn
	s.closed = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.conn == conn {
			s.conn = nil
		}
		s.mu.Unlock()
		_ = conn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			s.mu.RLock()
			closed := s.closed
			s.mu.RUnlock()
			if closed || errors.Is(err, net.ErrClosed) {
				return ErrServerClosed
			}
			return fmt.Errorf("dhcp4: read request: %w", err)
		}
		request, err := FromBytes(buf[:n])
		if err != nil || request.OpCode != OpCodeBootRequest {
			continue
		}
		s.requests.Add(1)
		rw := &responseWriter{
			conn:       conn,
			request:    request,
			clientPort: s.clientPort(),
		}
		go s.serve(request, rw)
	}
}

// serve runs the handler for one request. A panicking handler loses that
// request, not the server (or the program embedding it).
func (s *Server) serve(request *Message, rw *responseWriter) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("dhcp4: handler panic: %v\n%s", err, debug.Stack())
		}
	}()
	s.Handler.ServeDHCP(request, rw)
}

func (s *Server) clientPort() int {
	if s.ClientPort <= 0 {
		return 68
	}
	return s.ClientPort
}

func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return nil
	}
	err := conn.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (s *Server) LocalAddr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

func (s *Server) RequestCount() uint64 {
	return s.requests.Load()
}

func ListenAndServe(addr string, handler Handler) error {
	err := NewServer(addr, handler).ListenAndServe()
	if errors.Is(err, ErrServerClosed) {
		return nil
	}
	return err
}

type responseWriter struct {
	conn       *net.UDPConn
	request    *Message
	clientPort int
}

func (w *responseWriter) WriteResponse(resp *Message, responseOptions ...options.Option) error {
	resp.OpCode = OpCodeBootReply
	resp.Xid = w.request.Xid
	applyResponseOptions(resp, responseOptions)

	resp.Flags = w.request.Flags
	addr := w.destination(resp)
	_, err := w.conn.WriteTo(resp.Bytes(), &addr)
	return err
}

// flagBroadcast is the BOOTP broadcast flag a client sets when it cannot
// receive unicast before it is configured.
const flagBroadcast = 0x8000

// destination picks where a reply goes (RFC 2131, section 4.1): to the
// relay agent that forwarded the request; to a configured client at its
// address; otherwise broadcast. A client without an address cannot be
// reached by unicast without writing its ARP entry, which a UDP socket
// cannot do, so those replies are broadcast even without the flag, as
// most servers do. A DHCPNAK to a client is always broadcast.
func (w *responseWriter) destination(resp *Message) net.UDPAddr {
	nak := resp.MessageType() == options.DHCPNAK
	switch {
	case isSet(w.request.GatewayIPAddr):
		if nak {
			resp.Flags |= flagBroadcast
		}
		return net.UDPAddr{IP: w.request.GatewayIPAddr, Port: 67}
	case nak:
		return net.UDPAddr{IP: net.IPv4bcast, Port: w.clientPort}
	case isSet(w.request.ClientIPAddr):
		return net.UDPAddr{IP: w.request.ClientIPAddr, Port: w.clientPort}
	default:
		return net.UDPAddr{IP: net.IPv4bcast, Port: w.clientPort}
	}
}

func isSet(ip net.IP) bool { return ip != nil && !ip.Equal(net.IPv4zero) }

func applyResponseOptions(resp *Message, responseOptions []options.Option) {
	for _, option := range responseOptions {
		resp.SetOption(option)
		if serverID, ok := option.(options.ServerIdentifierOption); ok {
			resp.ServerIPAddr = serverID.ServerIdentifier
		}
	}
}

func (w *responseWriter) SendOffer(ip string, responseOptions ...options.Option) {
	_ = w.WriteResponse(NewOfferMessage(w.request, ip), responseOptions...)
}

func (w *responseWriter) SendAck(ip string, responseOptions ...options.Option) {
	_ = w.WriteResponse(NewAckMessage(w.request, ip), responseOptions...)
}

func (w *responseWriter) SendNak(reason string, responseOptions ...options.Option) {
	_ = w.WriteResponse(NewNakMessage(w.request, reason), responseOptions...)
}
