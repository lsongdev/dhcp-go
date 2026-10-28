package dhcp4

import "syscall"

// control hears every interface: Windows has no option tying a socket to
// one.
func control(fd uintptr, _ string, _ int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
