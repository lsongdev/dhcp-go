package options

import (
	"bytes"
	"fmt"
	"net"
)

// Option1 Subnet Mask
// The subnet mask option specifies the client's subnet mask as per RFC 950 [5].
// If both the subnet mask and the router option are specified in a DHCP reply, the subnet mask option MUST be first.
// The code for the subnet mask option is 1, and its length is 4 octets.
//
//	 Code   Len        Subnet Mask
//	+-----+-----+-----+-----+-----+-----+
//	|  1  |  4  |  m1 |  m2 |  m3 |  m4 |
//	+-----+-----+-----+-----+-----+-----+
type SubnetMaskOption struct {
	SubnetMask net.IP
}

func NewSubnetMaskOption(mask string) Option {
	return SubnetMaskOption{
		SubnetMask: net.ParseIP(mask),
	}
}

func (o SubnetMaskOption) Code() OptionCode {
	return OptionCodeSubnetMask
}

func (o SubnetMaskOption) Encode() []byte {
	return o.SubnetMask.To4()
}

func (o SubnetMaskOption) Decode(b []byte) Option {
	o.SubnetMask = b
	return o
}

func (o SubnetMaskOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(" Subnet Mask:")
	buf.WriteString(net.IP(o.SubnetMask).String())
	return buf.String()
}
