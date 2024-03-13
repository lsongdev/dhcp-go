package options

import (
	"bytes"
	"fmt"
)

// 3.17. Domain Name
// https://www.rfc-editor.org/rfc/rfc2132#section-3.17
// This option specifies the domain name that client should use when
// resolving hostnames via the Domain Name System.
// The code for this option is 15.  Its minimum length is 1.
// Code   Len        Domain Name
// +-----+-----+-----+-----+-----+-----+--
// |  15 |  n  |  d1 |  d2 |  d3 |  d4 |  ...
// +-----+-----+-----+-----+-----+-----+--
type DomainNameOption struct {
	Domain string `json:"domain"`
}

func NewDomainNameOption(domain string) Option {
	return DomainNameOption{
		Domain: domain,
	}
}

func (o DomainNameOption) Code() OptionCode {
	return OptionCodeDomainName
}

func (o DomainNameOption) Encode() []byte {
	return []byte(o.Domain)
}

func (o DomainNameOption) Decode(b []byte) Option {
	o.Domain = string(b)
	return o
}

func (o DomainNameOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf("Domain: %s", o.Domain))
	return buf.String()
}
