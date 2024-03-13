package options

import (
	"bytes"
	"fmt"
)

type MessageType uint8

const (
	DHCPDISCOVER         MessageType = 1 // https://www.rfc-editor.org/rfc/rfc2132#section-9.6
	DHCPOFFER            MessageType = 2
	DHCPREQUEST          MessageType = 3
	DHCPDECLINE          MessageType = 4
	DHCPACK              MessageType = 5
	DHCPNAK              MessageType = 6
	DHCPRELEASE          MessageType = 7
	DHCPINFORM           MessageType = 8
	DHCPFORCERENEW       MessageType = 9  // https://www.rfc-editor.org/rfc/rfc3203#section-4
	DHCPLEASEQUERY       MessageType = 10 // https://www.rfc-editor.org/rfc/rfc4388#section-6.1
	DHCPLEASEUNASSIGNED  MessageType = 11
	DHCPLEASEUNKNOWN     MessageType = 12
	DHCPLEASEACTIVE      MessageType = 13
	DHCPBULKLEASEQUERY   MessageType = 14 // https://www.rfc-editor.org/rfc/rfc6926#section-6.2.1
	DHCPLEASEQUERYDONE   MessageType = 15
	DHCPACTIVELEASEQUERY MessageType = 16 // https://www.rfc-editor.org/rfc/rfc7724#section-5.2.1
	DHCPLEASEQUERYSTATUS MessageType = 17
	DHCPTLS              MessageType = 18
)

func (o MessageType) String() string {
	switch o {
	case DHCPDISCOVER:
		return "Discover"
	case DHCPOFFER:
		return "Offer"
	case DHCPREQUEST:
		return "Request"
	case DHCPDECLINE:
		return "Decline"
	case DHCPACK:
		return "ACK"
	case DHCPNAK:
		return "NAK"
	case DHCPRELEASE:
		return "Release"
	case DHCPINFORM:
		return "Inform"
	default:
		return "Invalid"
	}
}

// Option53 DHCP Message Type(3 octets)
// Value   Message Type
// -----   ------------
// 1     DHCPDISCOVER
// 2     DHCPOFFER
// 3     DHCPREQUEST
// 4     DHCPDECLINE
// 5     DHCPACK
// 6     DHCPNAK
// 7     DHCPRELEASE
// Code   Len  Type
// +-----+-----+-----+
// |  53 |  1  | 1-7 |
// +-----+-----+-----+
type MessageTypeOption struct {
	Type MessageType `json:"type"`
}

func NewMessageType(t MessageType) Option {
	return MessageTypeOption{
		Type: t,
	}
}

func (o MessageTypeOption) Code() OptionCode {
	return OptionCodeMessageType
}

func (o MessageTypeOption) Encode() []byte {
	return []byte{uint8(o.Type)}
}

func (o MessageTypeOption) Decode(b []byte) Option {
	o.Type = MessageType(b[0])
	return o
}

func (o MessageTypeOption) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Option:(%d): ", o.Code()))
	buf.WriteString(fmt.Sprintf("Length: %d, ", len(o.Encode())))
	buf.WriteString(fmt.Sprintf("Type: %s", o.Type.String()))
	return buf.String()
}
