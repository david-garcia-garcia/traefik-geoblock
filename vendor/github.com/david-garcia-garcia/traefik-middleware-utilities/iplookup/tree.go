package iplookup

import "net"

// radixNode is one bit of a CIDR walk. An endpoint is a stored prefix.
type radixNode struct {
	isEndpoint bool
	prefixLen  int
	metadata   string
	left       *radixNode // bit 0
	right      *radixNode // bit 1
}

// ipRadixTree is one family's prefixes. It does not classify IPv4 vs IPv6.
type ipRadixTree struct {
	root *radixNode
}

// newIPRadixTree returns a tree whose root is not an endpoint.
func newIPRadixTree() *ipRadixTree {
	return &ipRadixTree{root: &radixNode{}}
}

// insert stores cidr's endpoint and metadata. added is true when the prefix was not already present.
func (tree *ipRadixTree) insert(cidr *net.IPNet, metadata string) (added bool) {
	ip, prefixLen := prefixWalk(cidr)

	current := tree.root
	// Walk each prefix bit; create missing children.
	for i := 0; i < prefixLen; i++ {
		bit := bitAt(ip, i)
		if bit == 0 {
			if current.left == nil {
				current.left = &radixNode{}
			}
			current = current.left
			continue
		}
		if current.right == nil {
			current.right = &radixNode{}
		}
		current = current.right
	}

	if current.isEndpoint {
		current.metadata = metadata
		return false
	}
	current.isEndpoint = true
	current.prefixLen = prefixLen
	current.metadata = metadata
	return true
}

// contains returns the longest-prefix endpoint that covers ip.
func (tree *ipRadixTree) contains(ip net.IP) (found bool, prefixLen int, metadata string) {
	walkIP, maxPrefixLen := familyWalk(ip)
	current := tree.root

	// Record each endpoint so the last one is the longest prefix.
	for i := 0; i < maxPrefixLen && current != nil; i++ {
		if current.isEndpoint {
			found = true
			prefixLen = current.prefixLen
			metadata = current.metadata
		}
		bit := bitAt(walkIP, i)
		if bit == 0 {
			current = current.left
			continue
		}
		current = current.right
	}

	if current != nil && current.isEndpoint {
		found = true
		prefixLen = current.prefixLen
		metadata = current.metadata
	}
	return found, prefixLen, metadata
}

// remove clears the endpoint for cidr and prunes nodes that hold nothing.
func (tree *ipRadixTree) remove(cidr *net.IPNet) bool {
	ip, prefixLen := prefixWalk(cidr)

	path := make([]*radixNode, 0, prefixLen+1)
	current := tree.root
	path = append(path, current)
	for i := 0; i < prefixLen; i++ {
		bit := bitAt(ip, i)
		var next *radixNode
		if bit == 0 {
			next = current.left
		} else {
			next = current.right
		}
		if next == nil {
			return false
		}
		path = append(path, next)
		current = next
	}

	if !current.isEndpoint {
		return false
	}
	current.isEndpoint = false
	current.prefixLen = 0
	current.metadata = ""

	// Drop empty leaves from the removed prefix back toward the root.
	for i := len(path) - 1; i > 0; i-- {
		node := path[i]
		if node.isEndpoint || node.left != nil || node.right != nil {
			break
		}
		parent := path[i-1]
		if parent.left == node {
			parent.left = nil
			continue
		}
		parent.right = nil
	}
	return true
}

// familyWalk maps ip onto the bit string used by insert and contains.
// IPv4 (including IPv4-mapped) walks the 4-byte To4() form from bit 0 on the
// v4 tree. Do not To16(); that allocates and the trees are already split.
func familyWalk(ip net.IP) (walk net.IP, maxPrefixLen int) {
	if v4 := ip.To4(); v4 != nil {
		return v4, 32
	}
	return ip, 128
}

// prefixWalk is the insert/remove walk for cidr: familyWalk of the network plus
// the prefix length to store. An IPv4-mapped CIDR (To4() non-nil and mask bits
// 128) remaps to IPv4 length ones-96 so the walk stays on the 4-byte IPv4
// form and membership matches net.IPNet.Contains. Native IPv4 (bits 32) and
// native IPv6 keep Mask.Size().
func prefixWalk(cidr *net.IPNet) (walk net.IP, prefixLen int) {
	walk, _ = familyWalk(cidr.IP)
	ones, bits := cidr.Mask.Size()
	prefixLen = ones
	if cidr.IP.To4() != nil && bits == 128 && ones >= 96 {
		prefixLen = ones - 96
	}
	return walk, prefixLen
}

// bitAt is the bit at actualBitPos in walk, most-significant bit first in each byte.
func bitAt(walk net.IP, actualBitPos int) byte {
	bytePos := actualBitPos / 8
	bitPos := 7 - (actualBitPos % 8)
	return (walk[bytePos] >> bitPos) & 1
}
