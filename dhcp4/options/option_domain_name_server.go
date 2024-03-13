package options

import (
	"bytes"
	"fmt"
	"net"
)

// Option6 Domain Name Server Option
// https://www.rfc-editor.org/rfc/rfc2132#section-3.8
// The domain name server option specifies a list of Domain Name System
//
//	(STD 13, RFC 1035 [8]) name servers available to the client.  Servers
//	SHOULD be listed in order of preference.
//	The code for the domain name server option is 6.  The minimum length
//	for this option is 4 octets, and the length MUST always be a multiple of 4.
//
//	 Code   Len         Address 1               Address 2
//	+-----+-----+-----+-----+-----+-----+-----+-----+--
//	|  6  |  n  |  a1 |  a2 |  a3 |  a4 |  a1 |  a2 |  ...
//	+-----+-----+-----+-----+-----+-----+-----+-----+--
type DomainNameServerOption struct {
	DomainNameServers []net.IP
}

func NewDomainNameServerOption(nameservers []string) Option {
	o := DomainNameServerOption{}
	for _, nameServer := range nameservers {
		o.DomainNameServers = append(o.DomainNameServers, net.ParseIP(nameServer))
	}
	return o
}

func (o DomainNameServerOption) Code() OptionCode {
	return OptionCodeDomainNameServer
}

func (o DomainNameServerOption) Encode() []byte {
	var buf bytes.Buffer
	for _, domainServer := range o.DomainNameServers {
		buf.Write(domainServer.To4())
	}
	return buf.Bytes()
}

func (o DomainNameServerOption) Decode(b []byte) Option {
	for i := 0; i < len(b); i += 4 {
		o.DomainNameServers = append(o.DomainNameServers, b[i:i+4])
	}
	return o
}

func (o DomainNameServerOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	for _, domainServer := range o.DomainNameServers {
		buf.WriteString(fmt.Sprintf(" DomainNameServer: %s", domainServer.String()))
	}
	return buf.String()
}
