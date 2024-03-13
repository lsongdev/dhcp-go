package options

import (
	"bytes"
	"fmt"
)

// Option59 Rebinding (T2) Time Value
// This option specifies the time interval from address assignment until
//
//	the client transitions to the REBINDING state.
//
//	The value is in units of seconds, and is specified as a 32-bit
//	unsigned integer.
//
//	The code for this option is 59, and its length is 4.
//
//	 Code   Len         T2 Interval
//	+-----+-----+-----+-----+-----+-----+
//	|  59 |  4  |  t1 |  t2 |  t3 |  t4 |
//	+-----+-----+-----+-----+-----+-----+
type RebindingTimeOption struct {
	RebindingTime uint32 `json:"rebinding_time"`
}

func NewRebindingTimeOption(rebindingTime uint32) RebindingTimeOption {
	return RebindingTimeOption{RebindingTime: rebindingTime}
}

func (o RebindingTimeOption) Code() OptionCode {
	return OptionCodeRebindingTime
}

func (o RebindingTimeOption) Encode() []byte {
	return Uint32ToBytes(o.RebindingTime)
}

func (o RebindingTimeOption) Decode(b []byte) Option {
	o.RebindingTime = BytesToUint32(b)
	return o
}

func (o RebindingTimeOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf(" RebindingTime: %d", o.RebindingTime))
	return buf.String()
}
