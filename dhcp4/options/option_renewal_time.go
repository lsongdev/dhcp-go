package options

import (
	"bytes"
	"fmt"
)

// Option58 Renewal (T1) Time Value
// This option specifies the time interval from address assignment until
//
//	the client transitions to the RENEWING state.
//	The value is in units of seconds, and is specified as a 32-bit
//	unsigned integer.
//
//	The code for this option is 58, and its length is 4.
//
//	 Code   Len         T1 Interval
//	+-----+-----+-----+-----+-----+-----+
//	|  58 |  4  |  t1 |  t2 |  t3 |  t4 |
//	+-----+-----+-----+-----+-----+-----+
type RenewalTimeOption struct {
	RenewalTime uint32 `json:"renewal_time"`
}

func NewRenewalTimeOption(t uint32) Option {
	return RenewalTimeOption{
		RenewalTime: t,
	}
}

func (o RenewalTimeOption) Code() OptionCode {
	return OptionCodeRenewalTime
}

func (o RenewalTimeOption) Encode() []byte {
	return Uint32ToBytes(o.RenewalTime)
}

func (o RenewalTimeOption) Decode(b []byte) Option {
	o.RenewalTime = BytesToUint32(b)
	return o
}

func (o RenewalTimeOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf("RenewalTime: %d", o.RenewalTime))
	return buf.String()
}
