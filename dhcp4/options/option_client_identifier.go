package options

import (
	"bytes"
	"fmt"
	"net"
)

// Option61 Client-identifier
// This option is used by DHCP clients to specify their unique
//
//	identifier.  DHCP servers use this value to index their database of
//	address bindings.  This value is expected to be unique for all
//	clients in an administrative domain.
//
// Identifiers consist of a type-value pair
//
//	It is expected that this field will typically contain a hardware type
//	and hardware address, but this is not required.  Current legal values
//	for hardware types are defined in [22].
//
// The code for this option is 61, and its minimum length is 2.
//
//	Code   Len   Type  Client-Identifier
//	+-----+-----+-----+-----+-----+---
//	|  61 |  n  |  t1 |  i1 |  i2 | ...
//	+-----+-----+-----+-----+-----+---
type ClientIdentifierOption struct {
	Type             uint8
	ClientIdentifier []byte
}

func NewClientIdentifierOption(id []byte, t uint8) Option {
	o := ClientIdentifierOption{
		Type:             t,
		ClientIdentifier: id,
	}
	return o
}

func NewClientIdentifierOptionWithMac(mac string) Option {
	addr, _ := net.ParseMAC(mac)
	return NewClientIdentifierOption(addr, 0x01)
}

func (o ClientIdentifierOption) Code() OptionCode {
	return OptionCodeClientIdentifier
}

func (o ClientIdentifierOption) Encode() []byte {
	var buf bytes.Buffer
	if o.Type != 0 {
		buf.WriteByte(o.Type)
	}
	buf.Write(o.ClientIdentifier)
	return buf.Bytes()
}

func (o ClientIdentifierOption) Decode(b []byte) Option {
	if b[0] == 0x01 { // FIXME: only 0x01 is supported
		o.Type = b[0]
		o.ClientIdentifier = b[1:]
	} else {
		o.ClientIdentifier = b
	}
	return o
}

func (o ClientIdentifierOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	if o.Type != 0 {
		buf.WriteString(fmt.Sprintf("Hardware Type: %d", o.Type))
		buf.WriteString(fmt.Sprintf("Hardware Addr: %s", net.HardwareAddr(o.ClientIdentifier).String()))
	} else {
		buf.WriteString(fmt.Sprintf("ClientIdentifier: %s", string(o.ClientIdentifier)))
	}
	return buf.String()
}
