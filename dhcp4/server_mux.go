package dhcp4

import (
	"log"
	"net"

	"github.com/song940/dhcp-go/dhcp4/options"
)

type IGetRequestedIP interface {
	GetRequestedIP() string
}

type IGetClientIP interface {
	GetClientIP() string
}

type DiscoverResponseWriter interface {
	SendOffer(ip string, options ...options.Option)
}

type RequestResponseWriter interface {
	SendAck(ip string, options ...options.Option)
	SendNak(reason string, options ...options.Option)
}

type ServerMuxHandler interface {
	HandleDiscover(discover *Message, rw DiscoverResponseWriter)
	HandleRequest(request IGetRequestedIP, rw RequestResponseWriter)
	HandleRenew(renew IGetClientIP, rw RequestResponseWriter)
	HandleDecline(decline IGetRequestedIP, rw RequestResponseWriter)
	HandleRelease(release IGetClientIP, rw RequestResponseWriter)
}

type DefaultServerMux struct {
	mux ServerMuxHandler
}

func NewDefaultServerMux(mux ServerMuxHandler) *DefaultServerMux {
	return &DefaultServerMux{
		mux: mux,
	}
}

func (d *DefaultServerMux) ServeDHCP(req *Message, rw ResponseWriter) {
	log.Println("Received:", req.MessageType(), req.String())
	var res *Message
	switch req.MessageType() {
	case options.DHCPDISCOVER:
		var rw DiscoverResponseWriter
		d.mux.HandleDiscover(req, rw)
	case options.DHCPREQUEST:
		var rw RequestResponseWriter
		d.mux.HandleRequest(req, rw)
	case options.DHCPDECLINE:
		var rw RequestResponseWriter
		d.mux.HandleDecline(req, rw)
	case options.DHCPRELEASE:
		var rw RequestResponseWriter
		d.mux.HandleRelease(req, rw)
	}
	if res != nil {
		res.ServerIPAddr = net.ParseIP("192.168.2.220")
		rw.WriteResponse(res)
	}
}

// func handleDiscover(req *Message) *Message {
// 	res := NewOfferMessage(req, "192.168.2.188")
// 	return res
// }

// func handleRequest(req *Message) *Message {
// 	// "Requested IP address" MUST (in SELECTING or INIT-REBOOT)
// 	ip := req.GetRequestedIP()
// 	if ip == "" && req.ClientIPAddr.String() != "0.0.0.0" {
// 		return handleRenew(req)
// 	}
// 	log.Println("Requested IP:", ip)
// 	res := NewAckMessage(req, ip)
// 	return res
// }

// func handleRenew(req *Message) *Message {
// 	// MUST have 'ciaddr' in RENEW
// 	// "Requested IP address" MUST NOT (in BOUND or RENEWING)
// 	ip := req.ClientIPAddr.String()
// 	log.Println("Renewed IP:", ip)
// 	res := NewAckMessage(req, ip)
// 	return res
// }

// func handleDecline(req *Message) *Message {
// 	// 'ciaddr' is 0 (DHCPDECLINE)
// 	// MUST have "Requested IP address"
// 	ip := req.GetRequestedIP()
// 	log.Println("Declined IP:", ip)
// 	return nil
// }

// func handleRelease(req *Message) {
// 	// MUST NOT have "Requested IP address"
// 	ip := req.ClientIPAddr.String()
// 	log.Println("Released IP:", ip)
// }
