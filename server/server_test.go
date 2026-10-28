package server

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/lsongdev/dhcp-go/dhcp4"
	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

func testNetwork() Network {
	_, subnet, _ := net.ParseCIDR("192.0.2.0/24")
	return Network{Interface: "eth0", IP: net.ParseIP("192.0.2.1").To4(), Subnet: subnet}
}

func testConfig() Config {
	return Config{Enabled: true, Interface: "eth0", RangeStart: "192.0.2.100", RangeEnd: "192.0.2.102", Router: "192.0.2.254", DNS: []string{"192.0.2.1"}, Domain: "lan", LeaseTime: 3600}
}

type memStore map[string]Lease

func (m memStore) SaveLease(l Lease) error     { m[l.IP] = l; return nil }
func (m memStore) DeleteLease(ip string) error { delete(m, ip); return nil }

const (
	macA = "00:00:5e:00:53:0a"
	macB = "00:00:5e:00:53:0b"
	macC = "00:00:5e:00:53:0c"
	macD = "00:00:5e:00:53:0d"
)

func TestConfigChecksAddresses(t *testing.T) {
	lookup := func(string) (Network, error) { return testNetwork(), nil }
	ok := testConfig()
	if err := ok.Normalize(lookup); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(*Config){
		"range outside subnet": func(c *Config) { c.RangeEnd = "192.0.3.10" },
		"reversed range":       func(c *Config) { c.RangeStart, c.RangeEnd = c.RangeEnd, c.RangeStart },
		"network address":      func(c *Config) { c.RangeStart = "192.0.2.0" },
		"broadcast address":    func(c *Config) { c.RangeEnd = "192.0.2.255" },
		"router elsewhere":     func(c *Config) { c.Router = "198.51.100.1" },
		"bad DNS":              func(c *Config) { c.DNS = []string{"dns.example"} },
		"bad domain":           func(c *Config) { c.Domain = "-lan" },
		"short lease":          func(c *Config) { c.LeaseTime = 10 },
		"enabled, no iface":    func(c *Config) { c.Interface = "" },
	}
	for name, change := range bad {
		c := testConfig()
		change(&c)
		if err := c.Normalize(lookup); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// A new installation: disabled, nothing set.
	if err := (&Config{}).Normalize(lookup); err != nil {
		t.Errorf("empty disabled config: %v", err)
	}
}

func TestPoolOffersAndBinds(t *testing.T) {
	store := memStore{}
	p := NewPool(testConfig(), testNetwork(), nil, nil, store)
	if p.Size() != 3 {
		t.Fatalf("size = %d", p.Size())
	}
	a, err := p.Offer(macA, nil)
	if err != nil || a.String() != "192.0.2.100" {
		t.Fatalf("offer A = %v, %v", a, err)
	}
	// A pending offer is not offered to anyone else.
	b, _ := p.Offer(macB, a)
	if b.String() != "192.0.2.101" {
		t.Fatalf("offer B = %v", b)
	}
	if _, err := p.Ack(macB, a, ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("B took A's offer: %v", err)
	}
	l, err := p.Ack(macA, a, "laptop")
	if err != nil || l.Hostname != "laptop" || store["192.0.2.100"].MAC != macA {
		t.Fatalf("ack A = %+v, %v; store %v", l, err, store)
	}
	// Renewal without a host name keeps the old one.
	if l, _ := p.Ack(macA, a, ""); l.Hostname != "laptop" {
		t.Fatalf("hostname after renewal = %q", l.Hostname)
	}
	// A client asks for its old address again and gets it.
	if again, _ := p.Offer(macA, nil); !again.Equal(a) {
		t.Fatalf("offer A again = %v", again)
	}
	// Outside the range is refused.
	if _, err := p.Ack(macC, net.ParseIP("192.0.2.50"), ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ack outside range: %v", err)
	}
}

func TestPoolExhaustionAndReuse(t *testing.T) {
	p := NewPool(testConfig(), testNetwork(), nil, nil, nil)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	for _, mac := range []string{macA, macB, macC} {
		ip, err := p.Offer(mac, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Ack(mac, ip, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Offer(macD, nil); !errors.Is(err, ErrExhausted) {
		t.Fatalf("offer to D with a full pool: %v", err)
	}
	// B releases; D gets B's address only once it is free.
	if err := p.Release(macB, net.ParseIP("192.0.2.101")); err != nil {
		t.Fatal(err)
	}
	ip, err := p.Offer(macD, nil)
	if err != nil || ip.String() != "192.0.2.101" {
		t.Fatalf("offer to D after release = %v, %v", ip, err)
	}
	if _, err := p.Ack(macD, ip, ""); err != nil {
		t.Fatal(err)
	}
	// B comes back: its old address is D's now, and nothing else is free.
	if _, err := p.Ack(macB, ip, ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("B renewed D's address: %v", err)
	}
	// After A's lease expires, the oldest expired address goes to B.
	now = now.Add(2 * time.Hour)
	if ip, err := p.Offer(macB, nil); err != nil || ip.String() != "192.0.2.100" {
		t.Fatalf("offer to B after expiry = %v, %v", ip, err)
	}
}

func TestPoolReservations(t *testing.T) {
	res := []Reservation{{MAC: macA, IP: "192.0.2.10"}, {MAC: macB, IP: "192.0.2.100"}}
	p := NewPool(testConfig(), testNetwork(), nil, res, nil)
	// A reservation may be outside the range.
	if ip, err := p.Offer(macA, nil); err != nil || ip.String() != "192.0.2.10" {
		t.Fatalf("offer to A = %v, %v", ip, err)
	}
	// A reserved address goes to nobody else, and its owner gets only it.
	if ip, _ := p.Offer(macC, net.ParseIP("192.0.2.100")); ip.String() == "192.0.2.100" {
		t.Fatal("offered B's reserved address to C")
	}
	if _, err := p.Ack(macB, net.ParseIP("192.0.2.102"), ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("B bound an address other than its reservation: %v", err)
	}
	// A lease restored for an address since reserved for someone else
	// stops being renewed.
	p = NewPool(testConfig(), testNetwork(), []Lease{{IP: "192.0.2.100", MAC: macC, ExpiresAt: time.Now().Add(time.Hour)}}, res, nil)
	if _, err := p.Ack(macC, net.ParseIP("192.0.2.100"), ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("C renewed B's reserved address: %v", err)
	}
}

func TestPoolDecline(t *testing.T) {
	p := NewPool(testConfig(), testNetwork(), nil, nil, nil)
	ip, _ := p.Offer(macA, nil)
	p.Ack(macA, ip, "")
	if err := p.Decline(macA, ip); err != nil {
		t.Fatal(err)
	}
	if next, _ := p.Offer(macA, nil); next.Equal(ip) {
		t.Fatal("declined address offered again")
	}
	if len(p.Leases()) != 0 {
		t.Fatalf("leases = %v", p.Leases())
	}
}

// recorder is a dhcp4.ResponseWriter that keeps the replies.
type recorder struct{ replies []*dhcp4.Message }

func (r *recorder) WriteResponse(m *dhcp4.Message, opts ...options.Option) error {
	for _, o := range opts {
		m.SetOption(o)
	}
	r.replies = append(r.replies, m)
	return nil
}
func (r *recorder) SendOffer(string, ...options.Option) {}
func (r *recorder) SendAck(string, ...options.Option)   {}
func (r *recorder) SendNak(string, ...options.Option)   {}

func (r *recorder) last(t *testing.T) *dhcp4.Message {
	t.Helper()
	if len(r.replies) == 0 {
		t.Fatal("no reply")
	}
	return r.replies[len(r.replies)-1]
}

func message(t options.MessageType, mac string, opts ...options.Option) *dhcp4.Message {
	m := dhcp4.NewMessage()
	m.SetMacAddress(mac)
	m.SetMessageType(t)
	for _, o := range opts {
		m.SetOption(o)
	}
	// Through the wire format, as the server receives it.
	m, _ = dhcp4.FromBytes(m.Bytes())
	return m
}

func TestServerHandshake(t *testing.T) {
	cfg := testConfig()
	s := New(cfg, testNetwork(), NewPool(cfg, testNetwork(), nil, nil, nil), nil)
	rw := &recorder{}

	s.ServeDHCP(message(options.DHCPDISCOVER, macA, options.NewHostNameOption("laptop")), rw)
	offer := rw.last(t)
	if offer.MessageType() != options.DHCPOFFER || offer.YourIPAddr.String() != "192.0.2.100" {
		t.Fatalf("offer: %s", offer)
	}
	for code, want := range map[options.OptionCode]string{
		options.OptionCodeServerIdentifier: "192.0.2.1", options.OptionCodeSubnetMask: "255.255.255.0",
		options.OptionCodeRouter: "192.0.2.254", options.OptionCodeDomainNameServer: "192.0.2.1",
		options.OptionCodeDomainName: "lan", options.OptionCodeLeaseTime: "3600",
	} {
		o := offer.GetOption(code)
		if o == nil || !strings.Contains(o.String(), want) {
			t.Errorf("option %d = %v, want %s", code, o, want)
		}
	}

	// Another server's offer was chosen: no reply, offer dropped.
	n := len(rw.replies)
	s.ServeDHCP(message(options.DHCPREQUEST, macA,
		options.NewServerIdentifierOption("192.0.2.2"), options.NewRequestedIPAddressOption("192.0.2.50")), rw)
	if len(rw.replies) != n {
		t.Fatal("answered a request meant for another server")
	}

	s.ServeDHCP(message(options.DHCPREQUEST, macA,
		options.NewServerIdentifierOption("192.0.2.1"), options.NewRequestedIPAddressOption("192.0.2.100"),
		options.NewHostNameOption("laptop")), rw)
	if ack := rw.last(t); ack.MessageType() != options.DHCPACK || ack.YourIPAddr.String() != "192.0.2.100" {
		t.Fatalf("ack: %s", ack)
	}
	if l := s.Pool.Leases(); len(l) != 1 || l[0].Hostname != "laptop" {
		t.Fatalf("leases = %+v", l)
	}

	// Renewal: ciaddr, no requested address, no server identifier.
	renew := message(options.DHCPREQUEST, macA)
	renew.ClientIPAddr = net.ParseIP("192.0.2.100").To4()
	s.ServeDHCP(renew, rw)
	if ack := rw.last(t); ack.MessageType() != options.DHCPACK {
		t.Fatalf("renewal: %s", ack)
	}

	// INIT-REBOOT from another network, and for another client's address.
	s.ServeDHCP(message(options.DHCPREQUEST, macB, options.NewRequestedIPAddressOption("10.0.0.5")), rw)
	if nak := rw.last(t); nak.MessageType() != options.DHCPNAK {
		t.Fatalf("foreign network: %s", nak)
	}
	s.ServeDHCP(message(options.DHCPREQUEST, macB, options.NewRequestedIPAddressOption("192.0.2.100")), rw)
	if nak := rw.last(t); nak.MessageType() != options.DHCPNAK {
		t.Fatalf("taken address: %s", nak)
	}

	// INFORM: options, no address, no lease time.
	inform := message(options.DHCPINFORM, macC)
	inform.ClientIPAddr = net.ParseIP("192.0.2.20").To4()
	s.ServeDHCP(inform, rw)
	ack := rw.last(t)
	if ack.MessageType() != options.DHCPACK || isSet(ack.YourIPAddr) || ack.GetOption(options.OptionCodeLeaseTime) != nil ||
		ack.GetOption(options.OptionCodeRouter) == nil {
		t.Fatalf("inform: %s", ack)
	}

	events := s.Events()
	if len(events) != 7 || events[0].Type != "inform" || events[len(events)-1].Result != "offer" {
		t.Fatalf("events = %+v", events)
	}
}
