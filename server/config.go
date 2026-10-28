// Package server is a DHCPv4 server for one network: the configuration and
// its checks, the lease pool, and the protocol handler on top of
// github.com/lsongdev/dhcp-go.
package server

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// ErrInvalid matches every configuration or reservation error
// (errors.Is); their messages are meant for the user as they are.
var ErrInvalid = errors.New("invalid")

type invalidError string

func (e invalidError) Error() string        { return string(e) }
func (e invalidError) Is(target error) bool { return target == ErrInvalid }

// Invalid returns an error matching ErrInvalid.
func Invalid(format string, args ...any) error {
	return invalidError(fmt.Sprintf(format, args...))
}

// Lease time bounds and default.
const (
	MinLeaseTime     = 2 * time.Minute
	MaxLeaseTime     = 90 * 24 * time.Hour
	DefaultLeaseTime = 12 * time.Hour
)

// Config configures the server. The server address and subnet come from
// the interface, so they always match the network the server answers on.
type Config struct {
	// Enabled starts the server. A new installation is disabled: a second
	// DHCP server on a network hands out conflicting addresses.
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Interface  string `json:"interface" yaml:"interface"`     // e.g. eth0
	RangeStart string `json:"range_start" yaml:"range_start"` // first address handed out
	RangeEnd   string `json:"range_end" yaml:"range_end"`     // last address handed out
	Router     string `json:"router" yaml:"router"`           // default gateway; empty sends none
	// DNS servers clients use; empty sends none.
	DNS       []string `json:"dns" yaml:"dns"`
	Domain    string   `json:"domain" yaml:"domain"`         // e.g. lan; empty sends none
	LeaseTime int      `json:"lease_time" yaml:"lease_time"` // seconds; 0 means DefaultLeaseTime
}

// Lease returns the lease duration.
func (c Config) Lease() time.Duration {
	if c.LeaseTime <= 0 {
		return DefaultLeaseTime
	}
	return time.Duration(c.LeaseTime) * time.Second
}

// Network is the interface the server answers on: its address, which is
// also the server identifier, and its subnet.
type Network struct {
	Interface string
	IP        net.IP // IPv4
	Subnet    *net.IPNet
}

// Broadcast returns the subnet's broadcast address.
func (n Network) Broadcast() net.IP {
	return toIP(toU32(n.Subnet.IP) | ^toU32(net.IP(n.Subnet.Mask)))
}

// LookupNetwork returns the first IPv4 address of the named interface.
func LookupNetwork(name string) (Network, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return Network{}, Invalid("interface %s: %v", name, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return Network{}, fmt.Errorf("interface %s: %w", name, err)
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
			ip := n.IP.To4()
			return Network{Interface: name, IP: ip, Subnet: &net.IPNet{IP: ip.Mask(n.Mask), Mask: n.Mask}}, nil
		}
	}
	return Network{}, Invalid("interface %s has no IPv4 address", name)
}

var domainRE = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// Normalize trims c and checks it against the network it would serve. A
// configuration without an interface is only valid while disabled.
func (c *Config) Normalize(lookup func(string) (Network, error)) error {
	c.Interface = strings.TrimSpace(c.Interface)
	c.RangeStart, c.RangeEnd = strings.TrimSpace(c.RangeStart), strings.TrimSpace(c.RangeEnd)
	c.Router = strings.TrimSpace(c.Router)
	c.Domain = strings.TrimSuffix(strings.TrimSpace(c.Domain), ".")
	dns := []string{}
	for _, d := range c.DNS {
		if d = strings.TrimSpace(d); d != "" {
			if parseIPv4(d) == nil {
				return Invalid("DNS server %q is not an IPv4 address", d)
			}
			dns = append(dns, d)
		}
	}
	c.DNS = dns
	if c.Domain != "" && (len(c.Domain) > 253 || !domainRE.MatchString(c.Domain)) {
		return Invalid("domain %q is not a valid domain name", c.Domain)
	}
	if c.LeaseTime != 0 && (c.Lease() < MinLeaseTime || c.Lease() > MaxLeaseTime) {
		return Invalid("lease time must be between %s and %s", MinLeaseTime, MaxLeaseTime)
	}
	if c.Interface == "" {
		if c.Enabled {
			return Invalid("choose an interface")
		}
		return nil
	}
	n, err := lookup(c.Interface)
	if err != nil {
		return err
	}
	return c.check(n)
}

// check validates the addresses against the network.
func (c *Config) check(n Network) error {
	start, end := parseIPv4(c.RangeStart), parseIPv4(c.RangeEnd)
	if start == nil || end == nil {
		return Invalid("the address range needs a first and a last IPv4 address")
	}
	if !n.Subnet.Contains(start) || !n.Subnet.Contains(end) {
		return Invalid("the address range must be inside %s, the network of %s", n.Subnet, n.Interface)
	}
	if toU32(start) > toU32(end) {
		return Invalid("the range's first address comes after its last")
	}
	if toU32(start) <= toU32(n.Subnet.IP) || toU32(end) >= toU32(n.Broadcast()) {
		return Invalid("the range must not include the network address %s or the broadcast address %s", n.Subnet.IP, n.Broadcast())
	}
	if c.Router != "" {
		r := parseIPv4(c.Router)
		if r == nil || !n.Subnet.Contains(r) {
			return Invalid("the router must be an address in %s", n.Subnet)
		}
	}
	return nil
}

func parseIPv4(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	return ip.To4()
}

func toU32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func toIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n)).To4()
}
