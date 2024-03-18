package examples

import (
	"log"

	"github.com/song940/dhcp-go/dhcp4"
	"github.com/song940/dhcp-go/dhcp4/options"
)

type MyServer struct {
}

// HandleDiscover implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleDiscover(request *dhcp4.Message, rw dhcp4.OfferWriter) {
	log.Println("Discover:", request.ClientHardwareAddr.String())
	rw.SendOffer("192.168.2.233",
		options.NewLeaseTimeOption(7776000),
		options.NewRenewalTimeOption(86400),
		options.NewSubnetMaskOption("255.255.255.0"),
		options.NewBroadcastAddress("192.168.2.255"),
		options.NewRouterOption([]string{"192.168.2.1"}),
		options.NewDomainNameServerOption([]string{"8.8.8.8", "8.8.4.4"}),
		options.NewDomainNameOption("lan"),
	)
}

// HandleRequest implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleRequest(request dhcp4.IGetRequestedIP, rw dhcp4.AckWriter) {
	ip := request.GetRequestedIP()
	mac := request.GetMacAddress()
	leaseTime := request.GetLeaseTime()
	log.Println("HandleRequest:", mac, ip, leaseTime)
	rw.SendAck(ip,
		options.NewLeaseTimeOption(43200),
		options.NewRenewalTimeOption(86400),
		options.NewSubnetMaskOption("255.255.255.0"),
		options.NewBroadcastAddress("192.168.2.255"),
		options.NewRouterOption([]string{"192.168.2.1"}),
		options.NewDomainNameServerOption([]string{"8.8.8.8", "8.8.4.4"}),
		options.NewDomainNameOption("lan"),
	)
}

// HandleDecline implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleDecline(request dhcp4.IGetRequestedIP, rw dhcp4.ResponseWriter) {
	ip := request.GetRequestedIP()
	log.Println("Declined IP:", ip)
	rw.SendAck(ip)
}

// HandleRenew implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleRenew(request dhcp4.IGetClientIP, rw dhcp4.AckWriter) {
	ip := request.GetClientIP()
	log.Println("Renewed IP:", ip)
	rw.SendAck(ip)
}

// HandleRelease implements dhcp4.ServerMuxHandler.
func (m MyServer) HandleRelease(request dhcp4.IGetClientIP, rw dhcp4.ResponseWriter) {
	ip := request.GetClientIP()
	log.Println("Released IP:", ip)
	rw.SendAck(ip)
}

func RunServer() {
	my := MyServer{}
	h := dhcp4.NewDefaultServerMux(my)
	dhcp4.ListenAndServe(":67", h)
}
