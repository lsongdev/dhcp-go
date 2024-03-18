package dhcp4

import (
	"net"

	"github.com/song940/dhcp-go/dhcp4/options"
)

type ServerMuxHandler interface {
	HandleDiscover(request *Message, rw OfferWriter)
	HandleRequest(request IGetRequestedIP, rw AckWriter)
	HandleRenew(request IGetClientIP, rw AckWriter)
	HandleRelease(request IGetClientIP, rw ResponseWriter)
	HandleDecline(request IGetRequestedIP, rw ResponseWriter)
}

type DefaultServerMux struct {
	h ServerMuxHandler
}

func NewDefaultServerMux(h ServerMuxHandler) *DefaultServerMux {
	return &DefaultServerMux{
		h: h,
	}
}

func (d *DefaultServerMux) ServeDHCP(req *Message, rw ResponseWriter) {
	// log.Println("Received:", req.MessageType(), req.String())
	switch req.MessageType() {
	case options.DHCPDISCOVER:
		d.h.HandleDiscover(req, rw)
	case options.DHCPREQUEST:
		if req.ClientIPAddr.Equal(net.IPv4zero) {
			d.h.HandleRequest(req, rw)
		} else {
			d.h.HandleRenew(req, rw)
		}
	case options.DHCPDECLINE:
		d.h.HandleDecline(req, rw)
	case options.DHCPRELEASE:
		d.h.HandleRelease(req, rw)
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
