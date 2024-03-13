package dhcp4

import (
	"net"

	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

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

func ListenAndServe(addr string, handler Handler) (err error) {
	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	for {
		buf := make([]byte, 2048)
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		request, err := FromBytes(buf[:n])
		if err != nil {
			return err
		}
		if request.OpCode != OpCodeBootRequest {
			continue
		}
		rw := &responseWriter{
			conn:    conn,
			request: request,
		}
		go handler.ServeDHCP(request, rw)
	}
}

type responseWriter struct {
	conn    *net.UDPConn
	request *Message
}

func (w *responseWriter) WriteResponse(resp *Message, options ...options.Option) (err error) {
	resp.OpCode = OpCodeBootReply
	resp.Xid = w.request.Xid
	for _, option := range options {
		resp.SetOption(option)
	}

	// Check broadcast flag from client request
	// If broadcast flag is set (0x8000), broadcast response; otherwise unicast
	broadcastFlag := (w.request.Flags & 0x8000) != 0
	var addr net.UDPAddr
	if broadcastFlag || resp.YourIPAddr.Equal(net.IPv4zero) {
		// Broadcast to 255.255.255.255:68
		addr = net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	} else {
		// Unicast to assigned IP address
		addr = net.UDPAddr{IP: resp.YourIPAddr, Port: 68}
	}
	_, err = w.conn.WriteTo(resp.Bytes(), &addr)
	return
}

// SendOffer implements DiscoverResponseWriter.
func (w *responseWriter) SendOffer(ip string, options ...options.Option) {
	reply := NewOfferMessage(w.request, ip)
	w.WriteResponse(reply, options...)
}

// SendAck implements ResponseWriter.
func (w *responseWriter) SendAck(ip string, options ...options.Option) {
	ack := NewAckMessage(w.request, ip)
	w.WriteResponse(ack, options...)
}

// SendNak implements ResponseWriter.
func (w *responseWriter) SendNak(reason string, options ...options.Option) {
	nak := NewNakMessage(w.request, reason)
	w.WriteResponse(nak, options...)
}
