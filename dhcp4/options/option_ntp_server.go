package options

import (
	"bytes"
	"fmt"
	"net"
)

// NetworkTimeProtocolServersOption is DHCP option 42.
type NetworkTimeProtocolServersOption struct {
	Servers []net.IP
}

func NewNetworkTimeProtocolServersOption(servers []string) Option {
	option := NetworkTimeProtocolServersOption{}
	for _, server := range servers {
		if ip := net.ParseIP(server).To4(); ip != nil {
			option.Servers = append(option.Servers, ip)
		}
	}
	return option
}

func (o NetworkTimeProtocolServersOption) Code() OptionCode {
	return OptionCodeNetworkTimeProtocolServers
}

func (o NetworkTimeProtocolServersOption) Encode() []byte {
	var buf bytes.Buffer
	for _, server := range o.Servers {
		buf.Write(server.To4())
	}
	return buf.Bytes()
}

func (o NetworkTimeProtocolServersOption) Decode(data []byte) Option {
	o.Servers = nil
	for len(data) >= 4 {
		o.Servers = append(o.Servers, net.IPv4(data[0], data[1], data[2], data[3]))
		data = data[4:]
	}
	return o
}

func (o NetworkTimeProtocolServersOption) String() string {
	return fmt.Sprintf("Option:(%d): NTP Servers: %v", o.Code(), o.Servers)
}
