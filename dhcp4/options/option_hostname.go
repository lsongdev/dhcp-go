package options

import (
	"bytes"
	"fmt"
)

// Option12 Host Name Option
// https://www.rfc-editor.org/rfc/rfc2132#section-3.14
// The code for this option is 12, and its minimum length is 1.
//
//	 Code   Len                 Host Name
//	+-----+-----+-----+-----+-----+-----+-----+-----+--
//	|  12 |  n  |  h1 |  h2 |  h3 |  h4 |  h5 |  h6 |  ...
//	+-----+-----+-----+-----+-----+-----+-----+-----+--
type HostNameOption struct {
	HostName string `json:"hostname"`
}

func NewHostNameOption(hostName string) Option {
	return HostNameOption{
		HostName: hostName,
	}
}

func (o HostNameOption) Code() OptionCode {
	return OptionCodeHostName
}

func (o HostNameOption) Encode() []byte {
	return []byte(o.HostName)
}

func (o HostNameOption) Decode(b []byte) Option {
	o.HostName = string(b)
	return o
}

func (o HostNameOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf("HostName: %s", o.HostName))
	return buf.String()
}
