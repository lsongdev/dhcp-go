package options

import (
	"bytes"
	"fmt"
	"net"
)

// Option3 Router Option
// The router option specifies a list of IP addresses for routers on the
//
//	client's subnet.  Routers SHOULD be listed in order of preference.
//	The code for the router option is 3.  The minimum length for the
//	router option is 4 octets, and the length MUST always be a multiple of 4.
//	 Code   Len         Address 1               Address 2
//	+-----+-----+-----+-----+-----+-----+-----+-----+--
//	|  3  |  n  |  a1 |  a2 |  a3 |  a4 |  a1 |  a2 |  ...
//	+-----+-----+-----+-----+-----+-----+-----+-----+--
type RouterOption struct {
	Routers []net.IP
}

func NewRouterOption(routers []string) Option {
	o := RouterOption{}
	for _, router := range routers {
		o.Routers = append(o.Routers, net.ParseIP(router))
	}
	return o
}

func (o RouterOption) Code() OptionCode {
	return OptionCodeRouter
}

func (o RouterOption) Encode() []byte {
	var buf bytes.Buffer
	for _, router := range o.Routers {
		buf.Write(router.To4())
	}
	return buf.Bytes()
}

func (o RouterOption) Decode(b []byte) Option {
	for i := 0; i < len(b); i += 4 {
		o.Routers = append(o.Routers, b[i:i+4])
	}
	return o
}

func (o RouterOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(" Routers:")
	for _, router := range o.Routers {
		buf.WriteString(router.String())
		buf.WriteString(" ")
	}
	return buf.String()
}
