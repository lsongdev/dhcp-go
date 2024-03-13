package options

import (
	"bytes"
	"fmt"
)

// Message
// This option is used by a DHCP server to provide an error message to a
// DHCP client in a DHCPNAK message in the event of a failure. A client
// may use this option in a DHCPDECLINE message to indicate the why the
// client declined the offered parameters.  The message consists of n
// octets of NVT ASCII text, which the client may display on an
// available output device.

// The code for this option is 56 and its minimum length is 1.

// Code   Len     Text
// +-----+-----+-----+-----+---
// |  56 |  n  |  c1 |  c2 | ...
// +-----+-----+-----+-----+---
type MessageOption struct {
	Text string `json:"text"`
}

func NewMessageOption(text string) Option {
	return MessageOption{
		Text: text,
	}
}

func (o MessageOption) Code() OptionCode {
	return OptionCodeMessage
}

func (o MessageOption) Encode() []byte {
	return []byte(o.Text)
}

func (o MessageOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf("Text: %s", o.Text))
	return buf.String()
}

func (o MessageOption) Decode(b []byte) Option {
	o.Text = string(b)
	return o
}
