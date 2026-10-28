package dhcp4

import (
	"bytes"
	"math/rand"
	"net"
	"testing"

	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

func TestFromBytesRoundTrip(t *testing.T) {
	m := NewDiscoverMessage()
	m.SetMacAddress("00:11:22:33:44:55")
	m.SetHostName("laptop")
	m.SetOption(options.NewRequestedIPAddressOption("192.0.2.10"))
	got, err := FromBytes(m.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.MessageType() != options.DHCPDISCOVER || got.GetMacAddress() != "00:11:22:33:44:55" ||
		got.GetHostName() != "laptop" || got.GetRequestedIP() != "192.0.2.10" || got.Xid != m.Xid {
		t.Fatalf("decoded %s", got)
	}
}

func TestFromBytesRejectsShortAndForeignData(t *testing.T) {
	if _, err := FromBytes(make([]byte, 100)); err != ErrShortMessage {
		t.Fatalf("short message: err = %v", err)
	}
	if _, err := FromBytes(make([]byte, 300)); err != ErrNotDHCPMessage {
		t.Fatalf("no magic cookie: err = %v", err)
	}
}

// Messages come from anyone on the network: no input may panic.
func TestFromBytesSurvivesMalformedInput(t *testing.T) {
	base := NewDiscoverMessage().Bytes()[:headerLength]
	cases := [][]byte{
		append(bytes.Clone(base), 53, 0),               // message type, no data
		append(bytes.Clone(base), 54, 1, 7),            // server identifier, 1 byte
		append(bytes.Clone(base), 3, 5, 1, 2, 3, 4, 5), // router, not a multiple of 4
		append(bytes.Clone(base), 61, 0),               // empty client identifier
		append(bytes.Clone(base), 12, 200, 'a'),        // length past the end
		append(bytes.Clone(base), 0, 0, 0, 255),        // padding
	}
	hlen := bytes.Clone(NewDiscoverMessage().Bytes())
	hlen[2] = 200 // hardware address length past its field
	cases = append(cases, hlen)
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		b := bytes.Clone(base)
		tail := make([]byte, rng.Intn(64))
		rng.Read(tail)
		cases = append(cases, append(b, tail...))
	}
	for _, c := range cases {
		m, err := FromBytes(c)
		if err == nil {
			_ = m.Bytes()
			_ = m.String()
		}
	}
	if m, _ := FromBytes(hlen); len(m.ClientHardwareAddr) != 16 {
		t.Fatalf("hardware address length = %d", len(m.ClientHardwareAddr))
	}
}

func TestBytesPutsMessageTypeFirst(t *testing.T) {
	m := NewAckMessage(NewRequestMessage(), "192.0.2.10")
	m.SetOption(options.NewSubnetMaskOption("255.255.255.0"))
	m.SetOption(options.NewRouterOption([]string{"192.0.2.1"}))
	b := m.Bytes()
	if b[headerLength] != byte(options.OptionCodeMessageType) {
		t.Fatalf("first option = %d", b[headerLength])
	}
	if !bytes.Equal(b, m.Bytes()) {
		t.Fatal("encoding is not deterministic")
	}
}

func TestReplyDestination(t *testing.T) {
	ip := net.ParseIP
	cases := []struct {
		name           string
		ciaddr, giaddr string
		nak            bool
		want           string
	}{
		{"new client", "0.0.0.0", "0.0.0.0", false, "255.255.255.255:68"},
		{"renewing client", "192.0.2.10", "0.0.0.0", false, "192.0.2.10:68"},
		{"nak", "192.0.2.10", "0.0.0.0", true, "255.255.255.255:68"},
		{"relayed", "0.0.0.0", "198.51.100.1", false, "198.51.100.1:67"},
	}
	for _, c := range cases {
		req := NewRequestMessage()
		req.ClientIPAddr, req.GatewayIPAddr = ip(c.ciaddr), ip(c.giaddr)
		w := &responseWriter{request: req, clientPort: 68}
		resp := NewAckMessage(req, "192.0.2.10")
		if c.nak {
			resp = NewNakMessage(req, "no")
		}
		addr := w.destination(resp)
		if got := addr.String(); got != c.want {
			t.Errorf("%s: destination = %s, want %s", c.name, got, c.want)
		}
	}
}
