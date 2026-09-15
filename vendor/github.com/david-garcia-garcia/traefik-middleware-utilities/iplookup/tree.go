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
	ip, bitStart, _ := familyWalk(cidr.IP)
	prefixLen, _ := cidr.Mask.Size()

	current := tree.root
	// Walk each prefix bit; create missing children.
	for i := 0; i < prefixLen; i++ {
		bit := bitAt(ip, bitStart+i)
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
	walkIP, bitStart, maxPrefixLen := familyWalk(ip)
	current := tree.root

	// Record each endpoint so the last one is the longest prefix.
	for i := 0; i < maxPrefixLen && current != nil; i++ {
		if current.isEndpoint {
			found = true
			prefixLen = current.prefixLen
			metadata = current.metadata
		}
		bit := bitAt(walkIP, bitStart+i)
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
	ip, bitStart, _ := familyWalk(cidr.IP)
	prefixLen, _ := cidr.Mask.Size()

	path := make([]*radixNode, 0, prefixLen+1)
	current := tree.root
	path = append(path, current)
	for i := 0; i < prefixLen; i++ {
		bit := bitAt(ip, bitStart+i)
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

// familyWalk maps ip onto the 16-byte walk used by insert and contains.
func familyWalk(ip net.IP) (walk net.IP, bitStart, maxPrefixLen int) {
	if ip.To4() != nil {
		return ip.To4().To16(), 96, 32
	}
	return ip, 0, 128
}

// bitAt is the bit at actualBitPos in walk, most-significant bit first in each byte.
func bitAt(walk net.IP, actualBitPos int) byte {
	bytePos := actualBitPos / 8
	bitPos := 7 - (actualBitPos % 8)
	return (walk[bytePos] >> bitPos) & 1
}
