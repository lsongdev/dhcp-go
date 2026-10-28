package dhcp4

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

// ListenInterface opens a UDP socket for a DHCP server on port (67) that
// only hears and answers the named interface. It binds the wildcard
// address, since clients without an address send to 255.255.255.255, and
// ties the socket to the interface: SO_BINDTODEVICE on Linux, IP_BOUND_IF
// on macOS. Elsewhere (and on Windows), or with an empty iface, it hears
// every interface.
func ListenInterface(iface string, port int) (*net.UDPConn, error) {
	index := 0
	if iface != "" {
		i, err := net.InterfaceByName(iface)
		if err != nil {
			return nil, fmt.Errorf("dhcp4: interface %s: %w", iface, err)
		}
		index = i.Index
	}
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		err := c.Control(func(fd uintptr) { serr = control(fd, iface, index) })
		if err != nil {
			return err
		}
		return serr
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("dhcp4: listen on UDP port %d: %w", port, err)
	}
	return pc.(*net.UDPConn), nil
}
