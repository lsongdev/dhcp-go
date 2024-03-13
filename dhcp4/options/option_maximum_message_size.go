package options

import (
	"bytes"
	"fmt"
)

// Option57 Maximum DHCP Message Size
// https://www.rfc-editor.org/rfc/rfc2132#section-9.10
// This option specifies the maximum length DHCP message that it is
//
//	willing to accept.  The length is specified as an unsigned 16-bit
//	integer.  A client may use the maximum DHCP message size option in
//	DHCPDISCOVER or DHCPREQUEST messages, but should not use the option in DHCPDECLINE messages.
//
// The code for this option is 57, and its length is 2.  The minimum
//
//	legal value is 576 octets.
//
//	 Code   Len     Length
//	+-----+-----+-----+-----+
//	|  57 |  2  |  l1 |  l2 |
//	+-----+-----+-----+-----+
type MaximumMessageSizeOption struct {
	MaximumMessageSize uint16 `json:"maximum_message_size"`
}

func NewMaximumMessageSizeOption(t uint16) Option {
	return MaximumMessageSizeOption{
		MaximumMessageSize: t,
	}
}

func (o MaximumMessageSizeOption) Code() OptionCode {
	return OptionCodeMaximumMessageSize
}

func (o MaximumMessageSizeOption) Encode() []byte {
	return Uint16ToBytes(o.MaximumMessageSize)
}

func (o MaximumMessageSizeOption) Decode(b []byte) Option {
	o.MaximumMessageSize = BytesToUint16(b)
	return o
}

func (o MaximumMessageSizeOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf(" MaximumMessageSize: %d", o.MaximumMessageSize))
	return buf.String()
}
