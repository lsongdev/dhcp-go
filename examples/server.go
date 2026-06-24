package examples

import (
	"fmt"
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
	PoolStart        string
	PoolEnd          string
}

// DefaultResponseOptions returns default DHCP response options based on server config.
func (c *ServerConfig) DefaultResponseOptions() []options.Option {
	opts := []options.Option{
		options.NewSubnetMaskOption(c.SubnetMask),
		options.NewLeaseTimeOption(uint32(c.LeaseDuration.Seconds())),
		options.NewRenewalTimeOption(uint32(c.RenewalTime.Seconds())),
	}
	if len(c.Router) > 0 {
		opts = append(opts, options.NewRouterOption(c.Router))
	}
	if c.DomainName != "" {
		opts = append(opts, options.NewDomainNameOption(c.DomainName))
	}
	if len(c.DNSServers) > 0 {
		opts = append(opts, options.NewDomainNameServerOption(c.DNSServers))
	}
	if c.BroadcastAddress != "" {
		opts = append(opts, options.NewBroadcastAddressOption(c.BroadcastAddress))
	}
	return opts
}

type MyServer struct {
	config *ServerConfig
	pool   *dhcp4.IPPool
}

// NewMyServer creates a new DHCP server with the given configuration.
func NewMyServer(config *ServerConfig) *MyServer {
	ip := config.ServerIP
	mask := net.IPMask(net.ParseIP(config.SubnetMask).To4())
	network := &net.IPNet{IP: ip.Mask(mask), Mask: mask}

	excluded := []net.IP{config.ServerIP}
	if len(config.Router) > 0 {
		excluded = append(excluded, net.ParseIP(config.Router[0]))
	}

	pool, err := dhcp4.NewIPPool(
		network,
		net.ParseIP(config.PoolStart),
		net.ParseIP(config.PoolEnd),
		excluded,
	)
	if err != nil {
		log.Fatalf("Failed to create IP pool: %v", err)
	}

	return &MyServer{
		config: config,
		pool:   pool,
	}
}

// HandleDiscover implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleDiscover(request *dhcp4.Message, rw dhcp4.OfferWriter) {
	mac := request.ClientHardwareAddr.String()
	log.Println("Discover:", mac)
	ip, err := m.pool.Allocate(mac)
	if err != nil {
		log.Printf("No IP available for %s: %v", mac, err)
		return
	}
	log.Printf("Offering IP %s to %s", ip, mac)
	rw.SendOffer(ip.String(), m.config.DefaultResponseOptions()...)
}

// HandleRequest implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleRequest(request dhcp4.IGetRequestedIP, rw dhcp4.AckWriter) {
	ip := request.GetRequestedIP()
	mac := request.GetMacAddress()
	leaseTime := request.GetLeaseTime()
	log.Println("HandleRequest:", mac, ip, leaseTime)

	allocated, err := m.pool.Allocate(mac)
	if err != nil {
		log.Printf("Cannot satisfy request for %s: %v", mac, err)
		return
	}
	allocatedStr := allocated.String()
	if allocatedStr != ip {
		log.Printf("Requested IP %s differs from allocated %s, using allocated", ip, allocatedStr)
		ip = allocatedStr
	}

	rw.SendAck(ip, m.config.DefaultResponseOptions()...)
}

// HandleDecline implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleDecline(request dhcp4.IGetRequestedIP, rw dhcp4.ResponseWriter) {
	ip := request.GetRequestedIP()
	log.Println("Declined IP:", ip)
	m.pool.Release(ip)
}

// HandleRenew implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleRenew(request dhcp4.IGetClientIP, rw dhcp4.AckWriter) {
	ip := request.GetClientIP()
	log.Println("Renewed IP:", ip)

	if !m.pool.IsLeased(ip) {
		log.Printf("IP %s is no longer available for renewal", ip)
		return
	}

	rw.SendAck(ip, m.config.DefaultResponseOptions()...)
}

// HandleRelease implements dhcp4.ServerMuxHandler.
func (m *MyServer) HandleRelease(request dhcp4.IGetClientIP, rw dhcp4.ResponseWriter) {
	ip := request.GetClientIP()
	log.Println("Released IP:", ip)
	m.pool.Release(ip)
}

func RunServer() {
	// Create server configuration
	config := &ServerConfig{
		ServerIP:         net.ParseIP("192.168.2.111"),
		ServerPort:       67,
		LeaseDuration:    24 * time.Hour,
		RenewalTime:      12 * time.Hour,
		SubnetMask:       "255.255.255.0",
		Router:           []string{"192.168.2.1"},
		DNSServers:       []string{"8.8.8.8", "8.8.4.4"},
		DomainName:       "lan",
		BroadcastAddress: "192.168.2.255",
		PoolStart:        "192.168.2.100",
		PoolEnd:          "192.168.2.200",
	}

	// Create server handler
	my := NewMyServer(config)
	log.Printf("IP pool: %s - %s", config.PoolStart, config.PoolEnd)
	h := dhcp4.NewDefaultServerMux(my)

	// Start server
	addr := fmt.Sprintf(":%d", config.ServerPort)
	log.Printf("Starting DHCP server on %s", addr)
	if err := dhcp4.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
