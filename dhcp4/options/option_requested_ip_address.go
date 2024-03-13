package options

import (
	"bytes"
	"fmt"
	"net"
)

// Option50 Requested IP Address
// https://www.rfc-editor.org/rfc/rfc2132#section-9.1
// This option is used in a client request (DHCPDISCOVER) to allow the
//
//	client to request that a particular IP address be assigned.
//	The code for this option is 50, and its length is 4.
//
//	 Code   Len          Address
//	+-----+-----+-----+-----+-----+-----+
//	|  50 |  4  |  a1 |  a2 |  a3 |  a4 |
//	+-----+-----+-----+-----+-----+-----+
type RequestedIPAddressOption struct {
	Address net.IP // 4 bytes
}

func NewRequestedIPAddressOption(ip string) Option {
	return RequestedIPAddressOption{
		Address: net.ParseIP(ip),
	}
}

func (o RequestedIPAddressOption) Code() OptionCode {
	return OptionCodeRequestedIPAddress
}

func (o RequestedIPAddressOption) Encode() []byte {
	return o.Address.To4()
}

func (o RequestedIPAddressOption) Decode(b []byte) Option {
	o.Address = b
	return o
}

func (o RequestedIPAddressOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf("Address: %s", o.Address.String()))
	return buf.String()
}
