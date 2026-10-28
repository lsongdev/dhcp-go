package server

import (
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrExhausted reports that no address is free.
	ErrExhausted = errors.New("no free address in the range")
	// ErrUnavailable reports an address this client may not have: outside
	// the range, another client's, reserved for another client, or
	// declined as in use.
	ErrUnavailable = errors.New("address not available to this client")
)

// Lease binds an address to a client.
type Lease struct {
	IP        string    `json:"ip"`
	MAC       string    `json:"mac"`
	Hostname  string    `json:"hostname"`
	ExpiresAt time.Time `json:"expires_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Reservation always gives a client (by MAC address) the same address,
// inside or outside the range.
type Reservation struct {
	ID          int64     `json:"id"`
	MAC         string    `json:"mac"`
	IP          string    `json:"ip"`
	Hostname    string    `json:"hostname"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Normalize checks a reservation's addresses and canonicalises them.
func (r *Reservation) Normalize() error {
	mac, err := net.ParseMAC(strings.TrimSpace(r.MAC))
	if err != nil || len(mac) != 6 {
		return Invalid("%q is not a MAC address", r.MAC)
	}
	r.MAC = mac.String()
	ip := parseIPv4(strings.TrimSpace(r.IP))
	if ip == nil {
		return Invalid("%q is not an IPv4 address", r.IP)
	}
	r.IP = ip.String()
	r.Hostname = strings.TrimSpace(r.Hostname)
	if r.Hostname != "" && !domainRE.MatchString(r.Hostname) {
		return Invalid("%q is not a valid host name", r.Hostname)
	}
	r.Description = strings.TrimSpace(r.Description)
	return nil
}

// LeaseStore persists bound leases, so clients keep their addresses across
// restarts.
type LeaseStore interface {
	SaveLease(Lease) error
	DeleteLease(ip string) error
}

// Offers are held this long for the client's DHCPREQUEST.
const offerHold = time.Minute

// declineHold keeps an address a client found in use (DHCPDECLINE) out of
// the pool for this long.
const declineHold = 10 * time.Minute

type offer struct {
	ip    uint32
	until time.Time
}

// Pool assigns addresses in a range. Every method is safe for concurrent
// use.
type Pool struct {
	mu         sync.Mutex
	network    Network
	start, end uint32
	excluded   map[uint32]bool // network, broadcast, server and router
	leaseTime  time.Duration
	store      LeaseStore

	leases   map[uint32]*Lease // bound leases, expired ones included
	byMAC    map[string]uint32
	reserved map[string]uint32 // MAC -> IP
	owners   map[uint32]string // reserved IP -> MAC
	offers   map[string]offer  // MAC -> pending offer
	declined map[uint32]time.Time

	now func() time.Time
}

// NewPool builds the pool for cfg on n, holding the leases persisted
// earlier (those no longer on the network are dropped) and reservations.
func NewPool(cfg Config, n Network, leases []Lease, reservations []Reservation, store LeaseStore) *Pool {
	p := &Pool{
		network:   n,
		start:     toU32(parseIPv4(cfg.RangeStart)),
		end:       toU32(parseIPv4(cfg.RangeEnd)),
		excluded:  map[uint32]bool{toU32(n.Subnet.IP): true, toU32(n.Broadcast()): true, toU32(n.IP): true},
		leaseTime: cfg.Lease(),
		store:     store,
		leases:    map[uint32]*Lease{},
		byMAC:     map[string]uint32{},
		offers:    map[string]offer{},
		declined:  map[uint32]time.Time{},
		now:       time.Now,
	}
	if r := parseIPv4(cfg.Router); r != nil {
		p.excluded[toU32(r)] = true
	}
	for _, l := range leases {
		ip := parseIPv4(l.IP)
		if ip == nil || !n.Subnet.Contains(ip) {
			continue
		}
		l := l
		p.leases[toU32(ip)] = &l
		p.byMAC[l.MAC] = toU32(ip)
	}
	p.SetReservations(reservations)
	return p
}

// SetReservations replaces the reservations. Reservations outside the
// network are ignored.
func (p *Pool) SetReservations(list []Reservation) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reserved, p.owners = map[string]uint32{}, map[uint32]string{}
	for _, r := range list {
		ip := parseIPv4(r.IP)
		if ip == nil || !p.network.Subnet.Contains(ip) {
			continue
		}
		p.reserved[r.MAC] = toU32(ip)
		p.owners[toU32(ip)] = r.MAC
	}
}

// available reports whether mac may have ip now. The caller holds p.mu.
func (p *Pool) available(mac string, ip uint32, now time.Time) bool {
	if p.excluded[ip] || !p.network.Subnet.Contains(toIP(ip)) {
		return false
	}
	if until, ok := p.declined[ip]; ok && now.Before(until) {
		return false
	}
	if owner, ok := p.owners[ip]; ok {
		return owner == mac
	}
	if _, ok := p.reserved[mac]; ok {
		return false // a client with a reservation gets only that address
	}
	if ip < p.start || ip > p.end {
		return false
	}
	if l, ok := p.leases[ip]; ok && l.MAC != mac && now.Before(l.ExpiresAt) {
		return false
	}
	for m, o := range p.offers {
		if o.ip == ip && m != mac && now.Before(o.until) {
			return false
		}
	}
	return true
}

// Offer picks an address for mac (DHCPDISCOVER) and holds it briefly:
// its reservation; the address offered or leased to it before; the one
// it asks for; a never-used address; then the one free the longest.
func (p *Pool) Offer(mac string, requested net.IP) (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	ip, err := p.choose(mac, requested, now)
	if err != nil {
		return nil, err
	}
	p.offers[mac] = offer{ip: ip, until: now.Add(offerHold)}
	return toIP(ip), nil
}

func (p *Pool) choose(mac string, requested net.IP, now time.Time) (uint32, error) {
	if ip, ok := p.reserved[mac]; ok {
		if p.available(mac, ip, now) {
			return ip, nil
		}
		return 0, ErrUnavailable
	}
	if o, ok := p.offers[mac]; ok && now.Before(o.until) && p.available(mac, o.ip, now) {
		return o.ip, nil
	}
	if ip, ok := p.byMAC[mac]; ok && p.available(mac, ip, now) {
		return ip, nil
	}
	if r := toU32(requested); requested != nil && p.available(mac, r, now) {
		return r, nil
	}
	var oldest uint32
	var oldestAt time.Time
	for ip := p.start; ip >= p.start && ip <= p.end; ip++ { // >= start stops at overflow
		if !p.available(mac, ip, now) {
			continue
		}
		l, used := p.leases[ip]
		if !used {
			return ip, nil
		}
		if oldest == 0 || l.ExpiresAt.Before(oldestAt) {
			oldest, oldestAt = ip, l.ExpiresAt
		}
	}
	if oldest != 0 {
		return oldest, nil
	}
	return 0, ErrExhausted
}

// Ack binds ip to mac (DHCPREQUEST) for the lease time and persists the
// lease. A client keeps the host name it sent before if it sends none.
func (p *Pool) Ack(mac string, ip net.IP, hostname string) (Lease, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	n := toU32(ip)
	if ip == nil || !p.available(mac, n, now) {
		return Lease{}, ErrUnavailable
	}
	if old, ok := p.byMAC[mac]; ok && old != n {
		p.drop(old)
	}
	if hostname == "" {
		if l, ok := p.leases[n]; ok && l.MAC == mac {
			hostname = l.Hostname
		}
	}
	if l, ok := p.leases[n]; ok && l.MAC != mac {
		delete(p.byMAC, l.MAC) // an expired lease of another client
	}
	l := &Lease{IP: toIP(n).String(), MAC: mac, Hostname: hostname, ExpiresAt: now.Add(p.leaseTime), UpdatedAt: now}
	p.leases[n], p.byMAC[mac] = l, n
	delete(p.offers, mac)
	return *l, p.save(*l)
}

// Cancel drops mac's pending offer: it chose another server.
func (p *Pool) Cancel(mac string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.offers, mac)
}

// Release ends mac's lease on ip (DHCPRELEASE). The record stays, expired,
// so the client gets the same address next time if it is still free.
func (p *Pool) Release(mac string, ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	l, ok := p.leases[toU32(ip)]
	if !ok || l.MAC != mac {
		return nil
	}
	now := p.now()
	if l.ExpiresAt.After(now) {
		l.ExpiresAt, l.UpdatedAt = now, now
	}
	return p.save(*l)
}

// Decline records that ip is in use by something else (DHCPDECLINE) and
// keeps it out of the pool for a while.
func (p *Pool) Decline(mac string, ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := toU32(ip)
	p.declined[n] = p.now().Add(declineHold)
	delete(p.offers, mac)
	if l, ok := p.leases[n]; ok && l.MAC == mac {
		return p.drop(n)
	}
	return nil
}

// Remove forgets the lease on ip, e.g. for a client that left for good.
// A client still using the address is refused at its next renewal only
// if another client took the address meanwhile.
func (p *Pool) Remove(ip string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := toU32(parseIPv4(ip))
	if _, ok := p.leases[n]; !ok {
		return nil
	}
	return p.drop(n)
}

// drop deletes a lease. The caller holds p.mu.
func (p *Pool) drop(ip uint32) error {
	l, ok := p.leases[ip]
	if !ok {
		return nil
	}
	delete(p.leases, ip)
	if p.byMAC[l.MAC] == ip {
		delete(p.byMAC, l.MAC)
	}
	if p.store == nil {
		return nil
	}
	return p.store.DeleteLease(l.IP)
}

func (p *Pool) save(l Lease) error {
	if p.store == nil {
		return nil
	}
	return p.store.SaveLease(l)
}

// Leases returns every lease, expired ones included, by address.
func (p *Pool) Leases() []Lease {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := make([]Lease, 0, len(p.leases))
	for _, l := range p.leases {
		list = append(list, *l)
	}
	sort.Slice(list, func(i, j int) bool { return toU32(parseIPv4(list[i].IP)) < toU32(parseIPv4(list[j].IP)) })
	return list
}

// Size returns how many addresses the range holds, excluding the server's,
// the router's, and the network and broadcast addresses.
func (p *Pool) Size() int {
	n := 0
	for ip := p.start; ip >= p.start && ip <= p.end; ip++ {
		if !p.excluded[ip] {
			n++
		}
	}
	return n
}
