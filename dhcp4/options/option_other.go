package options

import (
	"bytes"
	"fmt"
	"net"
)

// Option108 IPv6-Only Preferred Option
// Code:
// 8-bit identifier of the IPv6-Only Preferred option code as assigned by IANA: 108.
// The client includes the Code in the Parameter Request List in DHCPDISCOVER and DHCPREQUEST messages as described in Section 3.2.
// Length:
// 8-bit unsigned integer. The length of the option, excluding the Code and Length Fields.
// The server MUST set the length field to 4. The client MUST ignore the IPv6-Only Preferred option if the length field value is not 4.
// Value:
// 32-bit unsigned integer. The number of seconds for which the client should disable DHCPv4 (V6ONLY_WAIT configuration variable).
// If the server pool is explicitly configured with a V6ONLY_WAIT timer,
// the server MUST set the field to that configured value. Otherwise,
// the server MUST set it to zero. The client MUST process that field as described in Section 3.2.
//
// The client never sets this field, as it never sends the full option
// but includes the option code in the Parameter Request List as described in Section 3.2.
// 0                   1                   2                   3
//
//	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |     Code      |   Length      |           Value               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |         Value (cont.)         |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type Option108 struct {
	Value []byte
}

func (o Option108) Code() OptionCode {
	return 108
}

func (o Option108) Encode() []byte {
	return o.Value
}

func (o Option108) Decode(b []byte) Option {
	o.Value = b
	return o
}

func (o Option108) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString("IPv6-Only Preferred:")
	buf.WriteString(fmt.Sprintf(" Value: %x", o.Value))
	return buf.String()
}

/*
Option138
The DHCPv4 option for CAPWAP has the format shown in the following

	figure:
	      0                   1
	      0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	      |  option-code  | option-length |
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	      |                               |
	      +       AC IPv4 Address         +
	      |                               |
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	      |             ...               |
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	option-code:   OPTION_CAPWAP_AC_V4 (138)
	option-length:   Length of the 'options' field in octets; MUST be a
	   multiple of four (4).
	AC IPv4 Address:  IPv4 address of a CAPWAP AC that the WTP may use.
	   The ACs are listed in the order of preference for use by the WTP.
*/
type Option138 struct {
	Addresses []net.IP
}

func (o Option138) Code() OptionCode {
	return 138
}

func (o Option138) Encode() []byte {
	var buf bytes.Buffer
	for i := 0; i < len(o.Addresses); i += 4 {
		buf.Write(o.Addresses[i])
	}
	return buf.Bytes()
}

func (o Option138) Decode(b []byte) Option {
	for i := 0; i < len(b); i += 4 {
		o.Addresses = append(o.Addresses, b[i:i+4])
	}
	return o
}

func (o Option138) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString("AC IPv4 Address:")
	for i := 0; i < len(o.Addresses); i++ {
		buf.WriteString(fmt.Sprintf(" %s", o.Addresses[i].String()))
	}
	return buf.String()
}
