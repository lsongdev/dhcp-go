package dhcp4

import "syscall"

func control(fd uintptr, iface string, _ int) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return err
	}
	if iface == "" {
		return nil
	}
	return syscall.BindToDevice(int(fd), iface)
}
