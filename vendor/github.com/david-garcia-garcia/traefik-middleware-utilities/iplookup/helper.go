// Package iplookup is a Yaegi-safe CIDR store. IPv4 and IPv6 prefixes live on
// separate trees so a prefix cannot match the other family.
package iplookup

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
)

// Helper stores CIDR prefixes and returns the winning longest-prefix match.
// Callers pass net.IP they already selected; Helper does not read HTTP.
type Helper struct {
	mu       sync.RWMutex
	ipv4Tree *ipRadixTree
	ipv6Tree *ipRadixTree
	count    int
}

// New returns an empty Helper.
func New() *Helper {
	return &Helper{
		ipv4Tree: newIPRadixTree(),
		ipv6Tree: newIPRadixTree(),
	}
}

// treeFor returns the IPv4 tree when ip is IPv4 (including IPv4-mapped IPv6).
func (h *Helper) treeFor(ip net.IP) *ipRadixTree {
	if ip.To4() != nil {
		return h.ipv4Tree
	}
	return h.ipv6Tree
}

// AddCIDR stores cidr on the family tree for that network.
// A second store of the same canonical prefix replaces metadata and does not bump Count.
func (h *Helper) AddCIDR(cidr, metadata string) error {
	_, block, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("iplookup: parse CIDR %q: %w", cidr, err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.treeFor(block.IP).insert(block, metadata) {
		h.count++
	}
	return nil
}

// AddEntry stores a CIDR or a bare address. A bare address becomes HostCIDR of
// that address, so an IPv4-mapped literal such as ::ffff:10.0.0.1 is the IPv4
// host 10.0.0.1/32, not an IPv6 /32. Surrounding whitespace is ignored.
func (h *Helper) AddEntry(entry, metadata string) error {
	cidr, err := entryCIDR(entry)
	if err != nil {
		return err
	}
	return h.AddCIDR(cidr, metadata)
}

// HostCIDR is the single-address prefix for ip: /32 for IPv4 and IPv4-mapped
// IPv6 (written as dotted IPv4), /128 for native IPv6.
func HostCIDR(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String() + "/32"
	}
	return ip.String() + "/128"
}

// entryCIDR is entry as a CIDR string. Text with '/' is passed through for
// AddCIDR to parse; anything else must be one address.
func entryCIDR(entry string) (string, error) {
	trimmed := strings.TrimSpace(entry)
	if strings.IndexByte(trimmed, '/') >= 0 {
		return trimmed, nil
	}
	ip := net.ParseIP(trimmed)
	if ip == nil {
		return "", fmt.Errorf("iplookup: parse address or CIDR %q", entry)
	}
	return HostCIDR(ip), nil
}

// RemoveCIDR drops one stored prefix. Missing prefix returns false, nil error.
func (h *Helper) RemoveCIDR(cidr string) (removed bool, err error) {
	_, block, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, fmt.Errorf("iplookup: parse CIDR %q: %w", cidr, err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.treeFor(block.IP).remove(block) {
		return false, nil
	}
	h.count--
	return true, nil
}

// Contains returns the longest stored prefix of the same family that covers ipAddr.
func (h *Helper) Contains(ipAddr net.IP) (found bool, prefixLen int, metadata string, err error) {
	if ipAddr == nil {
		return false, 0, "", errors.New("iplookup: IP address is nil")
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	found, prefixLen, metadata = h.treeFor(ipAddr).contains(ipAddr)
	return found, prefixLen, metadata, nil
}

// Reset drops every stored prefix on both family trees.
func (h *Helper) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ipv4Tree = newIPRadixTree()
	h.ipv6Tree = newIPRadixTree()
	h.count = 0
}

// Count is the number of stored prefixes, not the number of AddCIDR calls.
func (h *Helper) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}
