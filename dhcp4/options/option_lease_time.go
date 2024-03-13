package options

import (
	"bytes"
	"fmt"
)

// Option51 IP Address Lease Time
// This option is used in a client request (DHCPDISCOVER or DHCPREQUEST)
//
//	to allow the client to request a lease time for the IP address.  In a
//	server reply (DHCPOFFER), a DHCP server uses this option to specify
//	the lease time it is willing to offer.
//
//	The time is in units of seconds, and is specified as a 32-bit
//	unsigned integer.
//
//	The code for this option is 51, and its length is 4.
//
//	 Code   Len         Lease Time
//	+-----+-----+-----+-----+-----+-----+
//	|  51 |  4  |  t1 |  t2 |  t3 |  t4 |
//	+-----+-----+-----+-----+-----+-----+
type LeaseTimeOption struct {
	LeaseTime uint32 `json:"lease_time"`
}

func NewLeaseTimeOption(t uint32) Option {
	return LeaseTimeOption{
		LeaseTime: t,
	}
}

func (o LeaseTimeOption) Code() OptionCode {
	return OptionCodeLeaseTime
}

func (o LeaseTimeOption) Encode() []byte {
	return Uint32ToBytes(o.LeaseTime)
}

func (o LeaseTimeOption) Decode(b []byte) Option {
	o.LeaseTime = BytesToUint32(b)
	return o
}

func (o LeaseTimeOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf(" IP Address Lease Time: %d", o.LeaseTime))
	return buf.String()
}
