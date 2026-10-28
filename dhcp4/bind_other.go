//go:build !linux && !darwin && !windows

package dhcp4

import "syscall"

// control has no portable way to tie the socket to an interface here: it
// hears every interface.
func control(fd uintptr, _ string, _ int) error {
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
