package dhcp4

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

type ClientConfig struct {
	Mac      string
	Server   string
	ClientIP string
	Hostname string
	Timeout  time.Duration
}

// DHCP client behavior
// https://datatracker.ietf.org/doc/html/rfc2131#section-4.4
type Client struct {
	conn   *net.UDPConn
	config *ClientConfig
}

func NewClient(config *ClientConfig) (c *Client, err error) {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	c = &Client{
		config: config,
	}
	addr := net.UDPAddr{IP: net.IPv4zero, Port: 68}
	c.conn, err = net.ListenUDP("udp4", &addr)
	return
}

func (c *Client) SetServer(addr string) {
	c.config.Server = addr
}

func (c *Client) SendMessage(addr *net.UDPAddr, message *Message) (err error) {
	_, err = c.conn.WriteTo(message.Bytes(), addr)
	return
}

func (c *Client) Close() {
	c.conn.Close()
}

// Receive returns a channel that receives DHCP response messages.
// It reads from the connection continuously and sends valid messages to the channel.
// The context can be used to cancel the receive process.
func (c *Client) Receive() (msg *Message, err error) {
	deadline := time.Now().Add(c.config.Timeout)
	err = c.conn.SetReadDeadline(deadline)
	if err != nil {
		return
	}
	respBuffer := make([]byte, 2048)
	n, _, err := c.conn.ReadFrom(respBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read from connection: %s", err)
	}
	msg, err = FromBytes(respBuffer[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to parse response message: %s", err)
	}
	if msg.OpCode != OpCodeBootReply {
		return nil, fmt.Errorf("received message with invalid opcode: %d", msg.OpCode)
	}
	return msg, nil
}

// ReceiveWithXid waits for a DHCP response message with the expected transaction ID.
func (c *Client) ReceiveWithXid(expectedXid uint32) (msg *Message, err error) {
	log.Println("waiting for response (xid:", expectedXid, ")")
	for {
		received, err := c.Receive()
		if err != nil {
			return nil, err
		}
		if received.Xid != expectedXid {
			log.Println("received mismatched xid:", received.Xid, "expected:", expectedXid)
			continue
		}
		log.Println("received matching response (xid:", received.Xid, ")")
		return received, nil
	}
}

func (c *Client) Discover() (offer *Message, err error) {
	message := NewDiscoverMessage()
	message.SetMacAddress(c.config.Mac)
	message.SetOption(options.NewHostNameOption(c.config.Hostname))
	// Set broadcast flag - server will broadcast response since client has no IP yet
	message.Flags = 0x8000
	// Save Xid to match response
	expectedXid := message.Xid
	destAddr := net.UDPAddr{IP: net.IPv4bcast, Port: 67}
	if err := c.SendMessage(&destAddr, message); err != nil {
		return nil, fmt.Errorf("failed to send discover message: %s", err)
	}
	return c.ReceiveWithXid(expectedXid)
}

func (c *Client) Request(offer *Message) (ack *Message, err error) {
	request := NewRequestMessage()
	ip := offer.YourIPAddr.String()
	request.SetMacAddress(c.config.Mac)
	request.SetOption(options.NewRequestedIPAddressOption(ip))
	request.SetOption(options.NewServerIdentifierOption(offer.ServerIPAddr.String()))
	// Set broadcast flag - server will broadcast response since client has no IP yet
	request.Flags = 0x8000
	// Save Xid to match response
	expectedXid := request.Xid
	addr := net.UDPAddr{IP: net.IPv4bcast, Port: 67}
	if err := c.SendMessage(&addr, request); err != nil {
		return nil, fmt.Errorf("failed to send request message: %s", err)
	}
	return c.ReceiveWithXid(expectedXid)
}

func (c *Client) Decline(offer *Message, reason string) (ack *Message, err error) {
	request := NewRequestMessage()
	request.Xid = offer.Xid
	request.SetMacAddress(c.config.Mac)
	request.SetOption(options.NewMessageType(options.DHCPDECLINE))
	request.SetOption(options.NewRequestedIPAddressOption(offer.YourIPAddr.String()))
	if reason != "" {
		request.SetOption(options.NewMessageOption(reason))
	}
	request.SetOption(options.NewServerIdentifierOption(offer.ServerIPAddr.String()))
	// Save Xid to match response
	expectedXid := request.Xid
	serverAddr := net.UDPAddr{IP: net.IPv4bcast, Port: 67}
	if err := c.SendMessage(&serverAddr, request); err != nil {
		return nil, fmt.Errorf("failed to send reboot message: %s", err)
	}
	return c.ReceiveWithXid(expectedXid)
}

func (c *Client) Renew() (ack *Message, err error) {
	if c.config.ClientIP == "" {
		return nil, fmt.Errorf("failed to get client IP")
	}
	request := NewRenewMessage(c.config.ClientIP)
	request.SetMacAddress(c.config.Mac)
	// Save Xid to match response
	expectedXid := request.Xid
	serverAddr := net.UDPAddr{IP: net.ParseIP(c.config.Server), Port: 67}
	if err := c.SendMessage(&serverAddr, request); err != nil {
		return nil, fmt.Errorf("failed to send renew message: %s", err)
	}
	return c.ReceiveWithXid(expectedXid)
}

func (c *Client) Release() (ack *Message, err error) {
	if c.config.ClientIP == "" {
		return nil, fmt.Errorf("failed to get client IP")
	}
	request := NewReleaseMessage(c.config.ClientIP)
	request.SetMacAddress(c.config.Mac)
	// Save Xid to match response
	expectedXid := request.Xid
	serverAddr := net.UDPAddr{IP: net.ParseIP(c.config.Server), Port: 67}
	if err := c.SendMessage(&serverAddr, request); err != nil {
		return nil, fmt.Errorf("failed to send release message: %s", err)
	}
	return c.ReceiveWithXid(expectedXid)
}

func (c *Client) Inform() (ack *Message, err error) {
	request := NewInformMessage()
	request.SetMacAddress(c.config.Mac)
	request.ServerIPAddr = net.ParseIP(c.config.Server)
	request.ClientIPAddr = net.ParseIP(c.config.ClientIP)
	// Save Xid to match response
	expectedXid := request.Xid
	serverAddr := net.UDPAddr{IP: request.ServerIPAddr, Port: 67}
	if err := c.SendMessage(&serverAddr, request); err != nil {
		return nil, fmt.Errorf("failed to send inform message: %s", err)
	}
	return c.ReceiveWithXid(expectedXid)
}
