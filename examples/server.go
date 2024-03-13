package examples

import (
	"log"
	"net"

	"github.com/song940/dhcp-go/dhcp4"
	"github.com/song940/dhcp-go/dhcp4/options"
)

func RunServer() {
	server := dhcp4.NewServer()
	h := &DefaultHandler{}
	server.ListenAndServe(":67", h)
}

type DefaultHandler struct {
}

func (d *DefaultHandler) ServeDHCP(req *dhcp4.Message, rw dhcp4.ResponseWriter) {
	log.Println("Received:", req.MessageType(), req.String())
	var res *dhcp4.Message
	switch req.MessageType() {
	case options.DHCPDISCOVER:
		res = handleDiscover(req)
	case options.DHCPREQUEST:
		res = handleRequest(req)
	case options.DHCPDECLINE:
		res = handleDecline(req)
	case options.DHCPRELEASE:
		handleRelease(req)
	}
	if res != nil {
		res.ServerIPAddr = net.ParseIP("192.168.2.220")
		rw.WriteResponse(res)
	}
}

func handleDiscover(req *dhcp4.Message) *dhcp4.Message {
	res := dhcp4.NewOfferMessage(req, "192.168.2.188")
	return res
}

func handleRequest(req *dhcp4.Message) *dhcp4.Message {
	// "Requested IP address" MUST (in SELECTING or INIT-REBOOT)
	ip := req.GetRequestedIP()
	if ip == "" && req.ClientIPAddr.String() != "0.0.0.0" {
		return handleRenew(req)
	}
	log.Println("Requested IP:", ip)
	res := dhcp4.NewAckMessage(req, ip)
	return res
}

func handleRenew(req *dhcp4.Message) *dhcp4.Message {
	// MUST have 'ciaddr' in RENEW
	// "Requested IP address" MUST NOT (in BOUND or RENEWING)
	ip := req.ClientIPAddr.String()
	log.Println("Renewed IP:", ip)
	res := dhcp4.NewAckMessage(req, ip)
	return res
}

func handleDecline(req *dhcp4.Message) *dhcp4.Message {
	// 'ciaddr' is 0 (DHCPDECLINE)
	// MUST have "Requested IP address"
	ip := req.GetRequestedIP()
	log.Println("Declined IP:", ip)
	return nil
}

func handleRelease(req *dhcp4.Message) {
	// MUST NOT have "Requested IP address"
	ip := req.ClientIPAddr.String()
	log.Println("Released IP:", ip)
}
