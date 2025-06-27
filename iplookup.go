package traefik_geoblock

import (
	"fmt"
	"net"
)

// radixNode represents a node in the IP radix tree
type radixNode struct {
	isEndpoint bool       // true if this node represents the end of a CIDR block
	prefixLen  int        // the prefix length of the CIDR block (if isEndpoint is true)
	left       *radixNode // for bit 0
	right      *radixNode // for bit 1
}

// ipRadixTree provides fast O(log k) IP block lookups where k is the IP bit length (32 for IPv4, 128 for IPv6)
type ipRadixTree struct {
	root *radixNode
}

// newIPRadixTree creates a new empty radix tree
func newIPRadixTree() *ipRadixTree {
	return &ipRadixTree{
		root: &radixNode{},
	}
}

// insert adds a CIDR block to the radix tree
func (tree *ipRadixTree) insert(cidr *net.IPNet) {
	ip := cidr.IP
	prefixLen, _ := cidr.Mask.Size()

	// Determine if this is IPv4 or IPv6
	isIPv4 := ip.To4() != nil
	var bitStart int

	if isIPv4 {
		// For IPv4, convert to 16-byte representation but note that
		// IPv4-mapped IPv6 has IPv4 bits at positions 96-127
		ip = ip.To4().To16()
		bitStart = 96 // IPv4 starts at bit 96 in IPv4-mapped IPv6
	} else {
		// For IPv6, use as-is
		bitStart = 0
	}

	current := tree.root

	// Walk through each bit of the IP up to the prefix length
	for i := 0; i < prefixLen; i++ {
		// Calculate actual bit position in the 16-byte array
		actualBitPos := bitStart + i
		bytePos := actualBitPos / 8
		bitPos := 7 - (actualBitPos % 8) // Most significant bit first

		// Extract the bit (0 or 1)
		bit := (ip[bytePos] >> bitPos) & 1

		// Go left for 0, right for 1
		if bit == 0 {
			if current.left == nil {
				current.left = &radixNode{}
			}
			current = current.left
		} else {
			if current.right == nil {
				current.right = &radixNode{}
			}
			current = current.right
		}
	}

	// Mark this node as an endpoint with the prefix length
	current.isEndpoint = true
	current.prefixLen = prefixLen
}

// contains checks if an IP address is contained in any of the CIDR blocks in the tree
// Returns (found, prefixLength) where found indicates if a match was found
// and prefixLength is the length of the matching CIDR block (for priority calculation)
func (tree *ipRadixTree) contains(ip net.IP) (bool, int) {
	// Determine if this is IPv4 or IPv6
	isIPv4 := ip.To4() != nil
	var bitStart, maxPrefixLen int

	if isIPv4 {
		// For IPv4, convert to 16-byte representation
		ip = ip.To4().To16()
		bitStart = 96     // IPv4 starts at bit 96 in IPv4-mapped IPv6
		maxPrefixLen = 32 // IPv4 addresses have max 32-bit prefixes
	} else {
		// For IPv6, use as-is
		bitStart = 0
		maxPrefixLen = 128 // IPv6 addresses have max 128-bit prefixes
	}

	current := tree.root
	longestMatch := 0
	found := false

	// Walk through each bit of the IP
	for i := 0; i < maxPrefixLen && current != nil; i++ {
		// Check if current node is an endpoint (represents a CIDR block)
		if current.isEndpoint {
			found = true
			longestMatch = current.prefixLen
			// Continue walking to find longest match (most specific CIDR)
		}

		// Calculate actual bit position in the 16-byte array
		actualBitPos := bitStart + i
		bytePos := actualBitPos / 8
		bitPos := 7 - (actualBitPos % 8) // Most significant bit first

		// Extract the bit (0 or 1)
		bit := (ip[bytePos] >> bitPos) & 1

		// Move to next node
		if bit == 0 {
			current = current.left
		} else {
			current = current.right
		}
	}

	// Check final node
	if current != nil && current.isEndpoint {
		found = true
		longestMatch = current.prefixLen
	}

	return found, longestMatch
}

// IpLookupHelper provides fast IP block lookups using radix trees
// Optimized for O(32) IPv4 and O(128) IPv6 lookups instead of O(n) linear search
type IpLookupHelper struct {
	tree *ipRadixTree
}

// NewIpLookupHelper creates a new IP lookup helper with the given CIDR block list
func NewIpLookupHelper(cidrBlocks []string) (*IpLookupHelper, error) {
	helper := &IpLookupHelper{
		tree: newIPRadixTree(),
	}

	// Parse and insert CIDR blocks
	for _, cidr := range cidrBlocks {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse error on CIDR %q: %v", cidr, err)
		}
		helper.tree.insert(block)
	}

	return helper, nil
}

// IsContained checks if an IP is contained in any of the CIDR blocks
// Returns (isContained, prefixLength, error)
func (helper *IpLookupHelper) IsContained(ipAddr net.IP) (bool, int, error) {
	found, prefixLen := helper.tree.contains(ipAddr)
	return found, prefixLen, nil
}
