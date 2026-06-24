package dhcp4

import (
	"errors"
	"net"
	"sync"
)

type IPPool struct {
	mu       sync.RWMutex
	network  *net.IPNet
	start    uint32
	end      uint32
	excluded map[uint32]bool
	leased   map[uint32]string
	macToIP  map[string]uint32
}

func NewIPPool(network *net.IPNet, start, end net.IP, excluded []net.IP) (*IPPool, error) {
	startU32 := ipToUint32(start)
	endU32 := ipToUint32(end)

	if !network.Contains(start) || !network.Contains(end) {
		return nil, errors.New("pool range is outside network")
	}
	if endU32 < startU32 {
		return nil, errors.New("end IP must be greater than or equal to start IP")
	}

	excludedMap := make(map[uint32]bool)
	for _, ip := range excluded {
		excludedMap[ipToUint32(ip)] = true
	}

	ones, bits := network.Mask.Size()
	ip := network.IP.To4()
	excludedMap[ipToUint32(net.IPv4(ip[0], ip[1], ip[2], ip[3]))] = true
	broadcast := net.IPv4(ip[0], ip[1], ip[2], ip[3])
	broadcast[3] |= byte(0xff >> (bits - ones))
	excludedMap[ipToUint32(broadcast)] = true

	return &IPPool{
		network:  network,
		start:    startU32,
		end:      endU32,
		excluded: excludedMap,
		leased:   make(map[uint32]string),
		macToIP:  make(map[string]uint32),
	}, nil
}

func (p *IPPool) Allocate(mac string) (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ip, ok := p.macToIP[mac]; ok {
		return uint32ToIP(ip), nil
	}

	for i := p.start; i <= p.end; i++ {
		if p.excluded[i] {
			continue
		}
		if _, ok := p.leased[i]; ok {
			continue
		}
		p.leased[i] = mac
		p.macToIP[mac] = i
		return uint32ToIP(i), nil
	}

	return nil, errors.New("no available IP addresses in pool")
}

func (p *IPPool) Release(ipStr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return
	}
	u32 := ipToUint32(ip)
	if mac, ok := p.leased[u32]; ok {
		delete(p.macToIP, mac)
		delete(p.leased, u32)
	}
}

func (p *IPPool) IsLeased(ipStr string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, ok := p.leased[ipToUint32(ip)]
	return ok
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}
