package options

import (
	"bytes"
	"fmt"
)

// Option55 Parameter Request List
// Code   Len   Option Codes
//
//	+-----+-----+-----+-----+---
//	|  55 |  n  |  c1 |  c2 | ...
//	+-----+-----+-----+-----+---
//
// The code for this option is 55.  Its minimum length is 1.
type ParameterRequestOption struct {
	Parameters []OptionCode `json:"parameters"`
}

func NewParameterRequestOption(codes []OptionCode) Option {
	return ParameterRequestOption{
		Parameters: codes,
	}
}

func (o ParameterRequestOption) Code() OptionCode {
	return OptionCodeParameterRequest
}

func (o ParameterRequestOption) Encode() []byte {
	var buf bytes.Buffer
	for _, parameter := range o.Parameters {
		buf.WriteByte(byte(parameter))
	}
	return buf.Bytes()
}

func (o ParameterRequestOption) Decode(b []byte) Option {
	o.Parameters = make([]OptionCode, 0)
	for i := 0; i < len(b); i++ {
		o.Parameters = append(o.Parameters, OptionCode(b[i]))
	}
	return o
}

func (o ParameterRequestOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(" Parameter Request List Item:")
	for _, parameter := range o.Parameters {
		buf.WriteString(fmt.Sprint(parameter))
		buf.WriteString(" ")
	}
	return buf.String()
}
