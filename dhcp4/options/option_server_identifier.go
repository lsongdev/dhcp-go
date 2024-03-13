package options

import (
	"bytes"
	"fmt"
	"net"
)

// Option54 Server Identifier
// The code for this option is 54, and its length is 4.
//
//	 Code   Len            Address
//	+-----+-----+-----+-----+-----+-----+
//	|  54 |  4  |  a1 |  a2 |  a3 |  a4 |
//	+-----+-----+-----+-----+-----+-----+
type ServerIdentifierOption struct {
	ServerIdentifier net.IP
}

func NewServerIdentifierOption(server string) Option {
	return ServerIdentifierOption{
		ServerIdentifier: net.ParseIP(server),
	}
}

func (o ServerIdentifierOption) Code() OptionCode {
	return OptionCodeServerIdentifier
}

func (o ServerIdentifierOption) Encode() []byte {
	return o.ServerIdentifier.To4()
}

func (o ServerIdentifierOption) Decode(b []byte) Option {
	o.ServerIdentifier = net.IPv4(b[0], b[1], b[2], b[3])
	return o
}

func (o ServerIdentifierOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf(" ServerIdentifier: %s", o.ServerIdentifier.String()))
	return buf.String()
}
