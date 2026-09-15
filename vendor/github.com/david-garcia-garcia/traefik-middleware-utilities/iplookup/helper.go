// Package iplookup is a Yaegi-safe CIDR store. IPv4 and IPv6 prefixes live on
// separate trees so a prefix cannot match the other family.
package iplookup

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

// Helper stores CIDR prefixes and returns the winning longest-prefix match.
// Callers pass net.IP they already selected; Helper does not read HTTP.
type Helper struct {
	mu       sync.Mutex
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
	h.mu.Lock()
	defer h.mu.Unlock()
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
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}
