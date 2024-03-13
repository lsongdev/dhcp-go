package options

import (
	"bytes"
	"fmt"
	"net"
)

// Broadcast Address Option
// https://www.rfc-editor.org/rfc/rfc2132#section-5.3
// This option specifies the broadcast address in use on the client's
// subnet.  Legal values for broadcast addresses are specified in
// section 3.2.1.3 of [4].

// The code for this option is 28, and its length is 4.

// Code   Len     Broadcast Address
// +-----+-----+-----+-----+-----+-----+
// |  28 |  4  |  b1 |  b2 |  b3 |  b4 |
// +-----+-----+-----+-----+-----+-----+
type BroadcastAddressOption struct {
	BroadcastAddress net.IP
}

func NewBroadcastAddress(broadcast string) Option {
	return BroadcastAddressOption{
		BroadcastAddress: net.ParseIP(broadcast),
	}
}

// GetCode implements Option.
func (b BroadcastAddressOption) Code() OptionCode {
	return OptionCodeBroadcastAddress
}

func (o BroadcastAddressOption) Encode() []byte {
	return o.BroadcastAddress.To4()
}

func (b BroadcastAddressOption) Decode(d []byte) Option {
	b.BroadcastAddress = d
	return b
}

func (o BroadcastAddressOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf("BroadcastAddress: %s", o.BroadcastAddress.String()))
	return buf.String()
}
