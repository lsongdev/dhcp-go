package examples

import (
	"log"
	"net"
	"time"

	"github.com/lsongdev/dhcp-go/dhcp4"
	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

// ServerConfig represents DHCP server configuration.
type ServerConfig struct {
	ServerIP         net.IP
	ServerPort       int
	LeaseDuration    time.Duration
	RenewalTime      time.Duration
	SubnetMask       string
	Router           []string
	DNSServers       []string
	DomainName       string
	BroadcastAddress string
}

// DefaultResponseOptions returns default DHCP response options based on server config.
func (c *ServerConfig) DefaultResponseOptions() []options.Option {
	opts := []options.Option{
		options.NewLeaseTimeOption(uint32(c.LeaseDuration.Seconds())),
		options.NewRenewalTimeOption(uint32(c.RenewalTime.Seconds())),
		options.NewSubnetMaskOption(c.SubnetMask),
	}
	if c.BroadcastAddress != "" {
		opts = append(opts, options.NewBroadcastAddress(c.BroadcastAddress))
	}
	if len(c.Router) > 0 {
		opts = append(opts, options.NewRouterOption(c.Router))
	}
	if len(c.DNSServers) > 0 {
		opts = append(opts, options.NewDomainNameServerOption(c.DNSServers))
	}
	if c.DomainName != "" {
		opts = append(opts, options.NewDomainNameOption(c.DomainName))
	}
	return opts
}

type MyServer struct {
	config *ServerConfig
}

// NewMyServer creates a new DHCP server with the given configuration.
func NewMyServer(config *ServerConfig) *MyServer {
	return &MyServer{
		config: config,
	}
}

// HandleDiscover implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleDiscover(request *dhcp4.Message, rw dhcp4.OfferWriter) {
	log.Println("Discover:", request.ClientHardwareAddr.String())
	rw.SendOffer("192.168.2.233", m.config.DefaultResponseOptions()...)
}

// HandleRequest implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleRequest(request dhcp4.IGetRequestedIP, rw dhcp4.AckWriter) {
	ip := request.GetRequestedIP()
	mac := request.GetMacAddress()
	leaseTime := request.GetLeaseTime()
	log.Println("HandleRequest:", mac, ip, leaseTime)
	rw.SendAck(ip, m.config.DefaultResponseOptions()...)
}

// HandleDecline implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleDecline(request dhcp4.IGetRequestedIP, rw dhcp4.ResponseWriter) {
	ip := request.GetRequestedIP()
	log.Println("Declined IP:", ip)
}

// HandleRenew implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleRenew(request dhcp4.IGetClientIP, rw dhcp4.AckWriter) {
	ip := request.GetClientIP()
	log.Println("Renewed IP:", ip)
	rw.SendAck(ip, m.config.DefaultResponseOptions()...)
}

// HandleRelease implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleRelease(request dhcp4.IGetClientIP, rw dhcp4.ResponseWriter) {
	ip := request.GetClientIP()
	log.Println("Released IP:", ip)
}

func RunServer() {
	// Create server configuration
	config := &ServerConfig{
		ServerIP:         net.ParseIP("192.168.2.128"),
		ServerPort:       67,
		LeaseDuration:    24 * time.Hour,
		RenewalTime:      12 * time.Hour,
		SubnetMask:       "255.255.255.0",
		Router:           []string{"192.168.2.1"},
		DNSServers:       []string{"8.8.8.8", "8.8.4.4"},
		DomainName:       "lan",
		BroadcastAddress: "192.168.2.255",
	}

	// Create server handler
	my := NewMyServer(config)
	h := dhcp4.NewDefaultServerMux(my)

	// Start server
	addr := net.UDPAddr{IP: config.ServerIP, Port: config.ServerPort}
	log.Printf("Starting DHCP server on %s", addr.String())
	if err := dhcp4.ListenAndServe(addr.String(), h); err != nil {
		log.Fatal(err)
	}
}
