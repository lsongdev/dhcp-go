package dhcp4

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/lsongdev/dhcp-go/dhcp4/options"
)

type testHandler struct {
	requests chan *Message
}

func (h *testHandler) ServeDHCP(req *Message, _ ResponseWriter) {
	h.requests <- req
}

func TestServerServesAndCloses(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	handler := &testHandler{requests: make(chan *Message, 1)}
	server := NewServer("", handler)
	done := make(chan error, 1)
	go func() { done <- server.Serve(conn) }()

	client, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write(NewDiscoverMessage().Bytes()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handler.requests:
	case <-time.After(time.Second):
		t.Fatal("handler did not receive request")
	}
	if server.RequestCount() != 1 {
		t.Fatalf("requests = %d, want 1", server.RequestCount())
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrServerClosed) {
			t.Fatalf("Serve error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestIPPoolAssign(t *testing.T) {
	_, network, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := NewIPPool(network, net.ParseIP("192.0.2.10"), net.ParseIP("192.0.2.20"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Assign("00:11:22:33:44:55", net.ParseIP("192.0.2.12")); err != nil {
		t.Fatal(err)
	}
	ip, err := pool.Allocate("00:11:22:33:44:55")
	if err != nil {
		t.Fatal(err)
	}
	if ip.String() != "192.0.2.12" {
		t.Fatalf("allocated %s, want 192.0.2.12", ip)
	}
}

func TestIPPoolExcludesBroadcastForWideSubnet(t *testing.T) {
	_, network, err := net.ParseCIDR("10.20.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := NewIPPool(network, net.ParseIP("10.20.255.255"), net.ParseIP("10.20.255.255"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Allocate("00:11:22:33:44:55"); err == nil {
		t.Fatal("broadcast address was allocated")
	}
}

func TestResponseWriterCopiesServerIdentifier(t *testing.T) {
	req := NewDiscoverMessage()
	resp := NewOfferMessage(req, "192.0.2.10")
	serverID := options.NewServerIdentifierOption("192.0.2.1")
	applyResponseOptions(resp, []options.Option{serverID})
	if got := resp.ServerIPAddr.String(); got != "192.0.2.1" {
		t.Fatalf("server IP = %s, want 192.0.2.1", got)
	}
}
