package options

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

type RawOption struct {
	code OptionCode // option code
	Data []byte     // option data
}

func NewRawOption(code OptionCode) Option {
	return RawOption{
		code: code,
	}
}

// GetCode implements Option.
func (o RawOption) Code() OptionCode {
	return o.code
}

func (o RawOption) Encode() []byte {
	return o.Data
}

func (o RawOption) Decode(data []byte) Option {
	o.Data = data
	return o
}

// String implements Option.
func (o RawOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(" Data:")
	buf.WriteString(hex.EncodeToString(o.Encode()))
	return buf.String()
}
