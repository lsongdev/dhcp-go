package dhcp4

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"

	L "github.com/song940/dhcp-go/dhcp4/options"
)

// OpCodeType represents DHCP message op code.
type OpCodeType uint8

// Constants representing DHCP op codes.
const (
	OpCodeBootRequest OpCodeType = 1
	OpCodeBootReply   OpCodeType = 2
)

func (opCode OpCodeType) String() string {
	switch opCode {
	case OpCodeBootRequest:
		return "BootRequest"
	case OpCodeBootReply:
		return "BootReply"
	default:
		return "Unknown"
	}
}

// MagicCookie is the DHCP magic cookie.
var MagicCookie = []byte{0x63, 0x82, 0x53, 0x63}

// Message represents a DHCP message.
type Message struct {
	OpCode             OpCodeType                `json:"op"`       // op(1 octet): Message op code / message type 1 = BOOTREQUEST, 2 = BOOTREPLY
	HardwareType       uint8                     `json:"htype"`    // htype(1 octet): Hardware address type, see ARP section in "Assigned Numbers" RFC; e.g., ’1’ = 10mb ethernet
	HardwareLength     uint8                     `json:"hlen"`     // hlen(1 octet): Hardware address length(e.g.  ’6’ for 10mb ethernet)
	Hops               uint8                     `json:"hops"`     // hops(1 octet): Client sets to zero, optionally used by relay agents when booting via a relay agent.
	Xid                uint32                    `json:"xid"`      // xid(4 octets): Transaction ID, a random number chosen by the client, used by the client and server to associate messages and responses between a client and a server
	Seconds            uint16                    `json:"secs"`     // secs(2 octets): Filled in by client, seconds elapsed since client began address acquisition or renewal process.
	Flags              uint16                    `json:"flags"`    // flags(2 octets): Bootp Flags
	ClientIPAddr       net.IP                    `json:"ciaddr"`   // ciaddr(4 octets): Client IP address; only filled in if client is in BOUND, RENEW or REBINDING state and can respond to ARP requests
	YourIPAddr         net.IP                    `json:"yiaddr"`   // yiaddr(4 octets): ’your’ (client) IP address
	ServerIPAddr       net.IP                    `json:"siaddr"`   // siaddr(4 octets): IP address of next server to use in bootstrap; returned in DHCPOFFER, DHCPACK by server.
	GatewayIPAddr      net.IP                    `json:"giaddr"`   // giaddr(4 octets): Relay agent IP address, used in booting via a relay agent
	ClientHardwareAddr net.HardwareAddr          `json:"chaddr"`   // chaddr(16 octets): Client hardware address(6 octets) + Client hardware address padding(10 octets)
	ServerHostName     string                    `json:"sname"`    // sname(64 octets): Optional server host name, null terminated string
	BootFileName       string                    `json:"filename"` // file(128 octets): Boot file name, null terminated string; "generic" name or null in DHCPDISCOVER, fully qualified directory-path name in DHCPOFFER
	MagicCookie        []byte                    `json:"cookie"`   // magicDhcp(4 octets): fixed value[63 92 53 63]
	Options            map[L.OptionCode]L.Option `json:"options"`  // options(var): Optional parameters field
}

// NewMessage creates a new DHCP message with default values.
func NewMessage() (m *Message) {
	m = &Message{
		OpCode:             OpCodeBootRequest,
		HardwareType:       1,
		HardwareLength:     6,
		Hops:               0,
		Xid:                rand.Uint32(),
		Seconds:            0,
		Flags:              0,
		ClientIPAddr:       net.IPv4zero,
		YourIPAddr:         net.IPv4zero,
		ServerIPAddr:       net.IPv4zero,
		GatewayIPAddr:      net.IPv4zero,
		ClientHardwareAddr: make([]byte, 16),
		ServerHostName:     "",
		BootFileName:       "",
		MagicCookie:        MagicCookie,
		Options:            make(map[L.OptionCode]L.Option),
	}
	return
}

// DHCP BootReply Message (for Server)
func NewReplyMessage(req *Message) *Message {
	reply := NewMessage()
	reply.OpCode = OpCodeBootReply
	// https://datatracker.ietf.org/doc/html/rfc2131#page-28
	// 'xid' from client DHCPDISCOVER or DHCPREQUEST message
	reply.Xid = req.Xid
	// 'giaddr' from client DHCPDISCOVER or DHCPREQUEST message
	reply.GatewayIPAddr = req.GatewayIPAddr
	// 'chaddr' from client DHCPDISCOVER or DHCPREQUEST message
	reply.ClientHardwareAddr = req.ClientHardwareAddr
	reply.HardwareType = req.HardwareType
	reply.HardwareLength = req.HardwareLength
	return reply
}

func NewOfferMessage(discover *Message, ip string) (offer *Message) {
	offer = NewReplyMessage(discover)
	offer.SetOption(L.NewMessageType(L.DHCPOFFER))
	offer.SetOption(L.NewLeaseTimeOption(7776000))
	// https: //datatracker.ietf.org/doc/html/rfc2131#page-28
	// IP address offered to client
	offer.YourIPAddr = net.ParseIP(ip)
	// Client identifier MUST NOT be included in DHCPOFFER and DHCPACK messages
	return
}

func NewAckMessage(req *Message, ip string) *Message {
	ack := NewReplyMessage(req)
	ack.SetOption(L.NewMessageType(L.DHCPACK))
	ack.SetOption(L.NewLeaseTimeOption(7776000))
	// IP address assigned to client
	// https://datatracker.ietf.org/doc/html/rfc2131#page-28
	ack.YourIPAddr = net.ParseIP(ip)
	return ack
}

func NewNakMessage(req *Message, reason string) *Message {
	nak := NewReplyMessage(req)
	nak.SetOption(L.NewMessageType(L.DHCPNAK))
	nak.SetTextMessage(reason)
	// MUST NOT have "Lease time"
	// MUST NOT have "Parameter request list"
	// https: //datatracker.ietf.org/doc/html/rfc2131#page-29
	return nak
}

// NewDiscoverMessage creates a new DHCP Discover message.
func NewDiscoverMessage() (m *Message) {
	m = NewMessage()
	m.OpCode = OpCodeBootRequest
	m.SetOption(L.NewMessageType(L.DHCPDISCOVER))
	return
}

// DHCP BootRequest Message (for Client)
// https://datatracker.ietf.org/doc/html/rfc2131#page-37
func NewRequestMessage() *Message {
	m := NewMessage()
	m.OpCode = OpCodeBootRequest
	m.SetOption(L.NewMessageType(L.DHCPREQUEST))
	return m
}

func NewRenewMessage(addr string) *Message {
	m := NewRequestMessage()
	m.ClientIPAddr = net.ParseIP(addr)
	return m
}

func NewReleaseMessage(addr string) *Message {
	m := NewMessage()
	m.OpCode = OpCodeBootRequest
	// client's network address (DHCPRELEASE)
	m.ClientIPAddr = net.ParseIP(addr)
	m.SetOption(L.NewMessageType(L.DHCPRELEASE))
	// MUST NOT have "Requested IP address"
	return m
}

func NewInformMessage() *Message {
	msg := NewMessage()
	msg.OpCode = OpCodeBootRequest
	msg.SetOption(L.NewMessageType(L.DHCPINFORM))
	return msg
}

// FromBytes decodes the DHCP message from bytes.
func FromBytes(data []byte) (m *Message, err error) {
	m = NewMessage()
	reader := bytes.NewReader(data)
	// Read fixed-length fields
	binary.Read(reader, binary.BigEndian, &m.OpCode)
	binary.Read(reader, binary.BigEndian, &m.HardwareType)
	binary.Read(reader, binary.BigEndian, &m.HardwareLength)
	binary.Read(reader, binary.BigEndian, &m.Hops)
	binary.Read(reader, binary.BigEndian, &m.Xid)
	binary.Read(reader, binary.BigEndian, &m.Seconds)
	binary.Read(reader, binary.BigEndian, &m.Flags)

	// Read IP addresses
	m.ClientIPAddr = readIP(reader)
	m.YourIPAddr = readIP(reader)
	m.ServerIPAddr = readIP(reader)
	m.GatewayIPAddr = readIP(reader)

	// Read ClientHardwareAddr
	m.ClientHardwareAddr = readBytes(reader, 16)
	m.ClientHardwareAddr = m.ClientHardwareAddr[:m.HardwareLength]
	//
	m.ServerHostName = readString(reader, 64)
	m.BootFileName = readString(reader, 128)
	m.MagicCookie = readBytes(reader, 4)
	for {
		code, err := reader.ReadByte()
		if err != nil {
			break
		}
		if code == 0xFF {
			break
		}
		length, _ := reader.ReadByte()
		data := readBytes(reader, int(length))
		c := L.OptionCode(code)
		m.Options[c] = L.ParseOption(c, data)
	}
	return
}

func readBytes(reader *bytes.Reader, length int) []byte {
	bytes := make([]byte, length)
	reader.Read(bytes)
	return bytes

}

func readIP(reader *bytes.Reader) net.IP {
	return readBytes(reader, net.IPv4len)
}

func readString(reader *bytes.Reader, length int) string {
	return string(bytes.TrimRight(readBytes(reader, length), "\x00"))
}

// Encode encodes the DHCP message to bytes.
func (m *Message) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, m.OpCode)
	binary.Write(buf, binary.BigEndian, m.HardwareType)
	binary.Write(buf, binary.BigEndian, m.HardwareLength)
	binary.Write(buf, binary.BigEndian, m.Hops)
	binary.Write(buf, binary.BigEndian, m.Xid)
	binary.Write(buf, binary.BigEndian, m.Seconds)
	binary.Write(buf, binary.BigEndian, m.Flags)

	buf.Write(m.ClientIPAddr.To4())
	buf.Write(m.YourIPAddr.To4())
	buf.Write(m.ServerIPAddr.To4())
	buf.Write(m.GatewayIPAddr.To4())

	hardwareBytes := make([]byte, 16)
	copy(hardwareBytes, m.ClientHardwareAddr)
	buf.Write(hardwareBytes)

	hostnameBytes := make([]byte, 64)
	copy(hostnameBytes, []byte(m.ServerHostName))
	buf.Write(hostnameBytes)

	filenameBytes := make([]byte, 128)
	copy(filenameBytes, []byte(m.BootFileName))
	buf.Write(filenameBytes)

	// MagicCookie
	buf.Write(m.MagicCookie)
	// write options
	for code, option := range m.Options {
		data := option.Encode()
		buf.WriteByte(byte(code))
		buf.WriteByte(byte(len(data)))
		buf.Write(data)
	}
	// EndOption
	buf.WriteByte(255)
	return buf.Bytes()
}

func (m Message) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("DHCP Message:\n")
	buffer.WriteString(fmt.Sprintf("  OpCode: %d (%s)\n", m.OpCode, m.OpCode.String()))
	buffer.WriteString(fmt.Sprintf("  HardwareType: %d\n", m.HardwareType))
	buffer.WriteString(fmt.Sprintf("  HardwareLength: %d\n", m.HardwareLength))
	buffer.WriteString(fmt.Sprintf("  Hops: %d\n", m.Hops))
	buffer.WriteString(fmt.Sprintf("  Xid: %d\n", m.Xid))
	buffer.WriteString(fmt.Sprintf("  Seconds: %d\n", m.Seconds))
	buffer.WriteString(fmt.Sprintf("  Flags: %d\n", m.Flags))
	buffer.WriteString(fmt.Sprintf("  ClientIPAddr: %s\n", m.ClientIPAddr))
	buffer.WriteString(fmt.Sprintf("  YourIPAddr: %s\n", m.YourIPAddr))
	buffer.WriteString(fmt.Sprintf("  ServerIPAddr: %s\n", m.ServerIPAddr))
	buffer.WriteString(fmt.Sprintf("  GatewayIPAddr: %s\n", m.GatewayIPAddr))
	buffer.WriteString(fmt.Sprintf("  ClientHardwareAddr: %s\n", m.ClientHardwareAddr))
	buffer.WriteString(fmt.Sprintf("  ServerHostName: %s\n", m.ServerHostName))
	buffer.WriteString(fmt.Sprintf("  BootFileName: %s\n", m.BootFileName))
	buffer.WriteString(fmt.Sprintf("  MagicCookie: %v\n", hex.EncodeToString(m.MagicCookie)))
	buffer.WriteString("  Options:\n")
	for code, option := range m.Options {
		buffer.WriteString(fmt.Sprintf("    Code: %d, %v\n", code, option))
	}
	return buffer.String()
}

func (m *Message) SetHardwareInfo(t uint8, h net.HardwareAddr) {
	m.HardwareType = t
	m.HardwareLength = uint8(len(h))
	m.ClientHardwareAddr = h
}

func (m *Message) SetOption(option L.Option) {
	m.Options[option.Code()] = option
}

func (m *Message) GetOption(code L.OptionCode) L.Option {
	return m.Options[code]
}

func (m *Message) GetHostName() string {
	option, ok := m.GetOption(L.OptionCodeHostName).(L.HostNameOption)
	if !ok {
		return ""
	}
	return option.HostName
}
func (m *Message) SetHostName(hostname string) {
	if hostname == "" {
		return
	}
	m.SetOption(L.NewHostNameOption(hostname))
}

func (m *Message) SetMacAddress(addr string) {
	if addr == "" {
		return
	}
	var hardwareTypeEthernet uint8 = 1
	mac, _ := net.ParseMAC(addr)
	m.SetHardwareInfo(hardwareTypeEthernet, mac)
}

func (m *Message) GetMacAddress() string {
	return m.ClientHardwareAddr.String()
}

func (m *Message) MessageType() L.MessageType {
	option, ok := m.GetOption(L.OptionCodeMessageType).(L.MessageTypeOption)
	if !ok {
		return L.MessageType(0)
	}
	return option.Type
}

func (m *Message) SetMessageType(t L.MessageType) {
	m.SetOption(L.NewMessageType(t))
}

func (m *Message) GetRequestedIP() string {
	option, ok := m.GetOption(L.OptionCodeRequestedIPAddress).(L.RequestedIPAddressOption)
	if !ok {
		return ""
	}
	return option.Address.String()
}

func (m *Message) GetClientIP() string {
	return m.ClientIPAddr.String()
}

func (m *Message) SetTextMessage(text string) {
	m.SetOption(L.NewMessageOption(text))
}

func (m *Message) GetLeaseTime() uint32 {
	option, ok := m.GetOption(L.OptionCodeLeaseTime).(L.LeaseTimeOption)
	if !ok {
		return 0
	}
	return option.LeaseTime
}

func (m *Message) SetServerIP(ip string) {
	m.ServerIPAddr = net.ParseIP(ip)
}
