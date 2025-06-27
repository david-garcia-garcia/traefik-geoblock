package traefik_geoblock

import (
	"net"
	"testing"
)

func TestIpLookupHelper_IPv4(t *testing.T) {
	cidrBlocks := []string{
		"192.168.1.0/24",  // Private network
		"10.0.0.0/8",      // Large private network
		"203.0.113.0/24",  // Test network
		"198.51.100.0/24", // Test network
		"192.168.1.10/32", // Single IP (more specific than /24)
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		t.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	tests := []struct {
		name           string
		ip             string
		shouldMatch    bool
		expectedPrefix int
	}{
		{"IP in 192.168.1.0/24", "192.168.1.5", true, 24},
		{"Specific IP 192.168.1.10/32", "192.168.1.10", true, 32}, // Should match most specific
		{"IP in 10.0.0.0/8", "10.5.10.15", true, 8},
		{"IP in test network", "203.0.113.100", true, 24},
		{"IP not in any range", "8.8.8.8", false, 0},
		{"IP not in any range", "1.1.1.1", false, 0},
		{"Edge of range", "192.168.1.255", true, 24},
		{"Just outside range", "192.168.2.1", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Invalid IP address: %s", tt.ip)
			}

			found, prefixLen, err := helper.IsContained(ip)
			if err != nil {
				t.Errorf("IsContained returned error: %v", err)
			}

			if found != tt.shouldMatch {
				t.Errorf("IsContained(%s) = %v, want %v", tt.ip, found, tt.shouldMatch)
			}

			if found && prefixLen != tt.expectedPrefix {
				t.Errorf("IsContained(%s) prefix = %d, want %d", tt.ip, prefixLen, tt.expectedPrefix)
			}
		})
	}
}

func TestIpLookupHelper_IPv6(t *testing.T) {
	cidrBlocks := []string{
		"2001:db8::/32",          // Test network
		"fe80::/10",              // Link-local
		"::1/128",                // Localhost
		"2001:db8:85a3::/48",     // More specific subnet
		"2001:db8:85a3:8d3::/64", // Even more specific
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		t.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	tests := []struct {
		name           string
		ip             string
		shouldMatch    bool
		expectedPrefix int
	}{
		{"IPv6 localhost", "::1", true, 128},
		{"IPv6 in 2001:db8::/32", "2001:db8:1234:5678::1", true, 32},
		{"IPv6 in more specific subnet", "2001:db8:85a3:1234::1", true, 48},     // Should match /48, not /32
		{"IPv6 in most specific subnet", "2001:db8:85a3:8d3:1234::1", true, 64}, // Should match /64
		{"IPv6 link-local", "fe80::1", true, 10},
		{"IPv6 not in any range", "2001:db9::1", false, 0},
		{"IPv6 global unicast not in range", "2a00:1450:4001::1", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Invalid IP address: %s", tt.ip)
			}

			found, prefixLen, err := helper.IsContained(ip)
			if err != nil {
				t.Errorf("IsContained returned error: %v", err)
			}

			if found != tt.shouldMatch {
				t.Errorf("IsContained(%s) = %v, want %v", tt.ip, found, tt.shouldMatch)
			}

			if found && prefixLen != tt.expectedPrefix {
				t.Errorf("IsContained(%s) prefix = %d, want %d (most specific match)", tt.ip, prefixLen, tt.expectedPrefix)
			}
		})
	}
}

func TestIpLookupHelper_MixedIPv4AndIPv6(t *testing.T) {
	cidrBlocks := []string{
		"192.168.1.0/24", // IPv4
		"2001:db8::/32",  // IPv6
		"10.0.0.0/8",     // IPv4 large network
		"::1/128",        // IPv6 localhost
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		t.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	tests := []struct {
		name        string
		ip          string
		shouldMatch bool
	}{
		{"IPv4 in range", "192.168.1.100", true},
		{"IPv4 not in range", "8.8.8.8", false},
		{"IPv6 in range", "2001:db8::1", true},
		{"IPv6 not in range", "2001:db9::1", false},
		{"IPv6 localhost", "::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Invalid IP address: %s", tt.ip)
			}

			found, _, err := helper.IsContained(ip)
			if err != nil {
				t.Errorf("IsContained returned error: %v", err)
			}

			if found != tt.shouldMatch {
				t.Errorf("IsContained(%s) = %v, want %v", tt.ip, found, tt.shouldMatch)
			}
		})
	}
}

func TestIpLookupHelper_EmptyHelper(t *testing.T) {
	helper, err := NewIpLookupHelper([]string{})
	if err != nil {
		t.Fatalf("Failed to create empty IpLookupHelper: %v", err)
	}

	testIPs := []string{"192.168.1.1", "8.8.8.8", "::1", "2001:db8::1"}

	for _, ipStr := range testIPs {
		t.Run("Empty_helper_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("Invalid IP address: %s", ipStr)
			}

			found, prefixLen, err := helper.IsContained(ip)
			if err != nil {
				t.Errorf("IsContained returned error: %v", err)
			}

			if found {
				t.Errorf("Empty helper should not match any IP, but matched %s with prefix %d", ipStr, prefixLen)
			}
		})
	}
}

func TestIpLookupHelper_InvalidCIDR(t *testing.T) {
	invalidCIDRs := []string{
		"invalid-cidr",
		"192.168.1.0/33", // Invalid prefix for IPv4
		"2001:db8::/129", // Invalid prefix for IPv6
		"192.168.1",      // Missing prefix
		"",               // Empty string
	}

	for _, cidr := range invalidCIDRs {
		t.Run("Invalid_CIDR_"+cidr, func(t *testing.T) {
			_, err := NewIpLookupHelper([]string{cidr})
			if err == nil {
				t.Errorf("Expected error for invalid CIDR %s, but got none", cidr)
			}
		})
	}
}

func TestIpLookupHelper_OverlappingRanges(t *testing.T) {
	cidrBlocks := []string{
		"192.168.0.0/16",  // Large network
		"192.168.1.0/24",  // Subnet of above
		"192.168.1.10/32", // Single IP within subnet
		"10.0.0.0/8",      // Different large network
		"10.1.0.0/16",     // Subnet of above
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		t.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	tests := []struct {
		name           string
		ip             string
		expectedPrefix int // Should match the most specific (longest prefix)
	}{
		{"Most specific match", "192.168.1.10", 32},
		{"Subnet match", "192.168.1.5", 24},
		{"Large network match", "192.168.2.1", 16},
		{"Other network subnet", "10.1.1.1", 16},
		{"Other network general", "10.2.1.1", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Invalid IP address: %s", tt.ip)
			}

			found, prefixLen, err := helper.IsContained(ip)
			if err != nil {
				t.Errorf("IsContained returned error: %v", err)
			}

			if !found {
				t.Errorf("IsContained(%s) = false, expected true", tt.ip)
			}

			if prefixLen != tt.expectedPrefix {
				t.Errorf("IsContained(%s) prefix = %d, want %d (most specific)", tt.ip, prefixLen, tt.expectedPrefix)
			}
		})
	}
}

func TestIpLookupHelper_EdgeCases(t *testing.T) {
	cidrBlocks := []string{
		"0.0.0.0/0",          // All IPv4
		"255.255.255.255/32", // Single IP at edge
		"::/0",               // All IPv6
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		t.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	tests := []struct {
		name string
		ip   string
	}{
		{"IPv4 any", "1.2.3.4"},
		{"IPv4 edge", "255.255.255.255"},
		{"IPv4 zero", "0.0.0.0"},
		{"IPv6 any", "2001:db8::1"},
		{"IPv6 localhost", "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Invalid IP address: %s", tt.ip)
			}

			found, _, err := helper.IsContained(ip)
			if err != nil {
				t.Errorf("IsContained returned error: %v", err)
			}

			if !found {
				t.Errorf("IsContained(%s) = false, expected true (should match catch-all)", tt.ip)
			}
		})
	}
}

// Benchmark to compare radix tree performance
func BenchmarkIpLookupHelper_IPv4_Small(b *testing.B) {
	cidrBlocks := []string{
		"192.168.1.0/24",
		"10.0.0.0/8",
		"203.0.113.0/24",
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		b.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	ip := net.ParseIP("192.168.1.100")
	if ip == nil {
		b.Fatalf("Invalid IP address")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.IsContained(ip)
	}
}

func BenchmarkIpLookupHelper_IPv4_Large(b *testing.B) {
	// Create many CIDR blocks to test scalability
	var cidrBlocks []string
	for i := 0; i < 1000; i++ {
		cidrBlocks = append(cidrBlocks, "10.0.0.0/8")     // Same network repeated
		cidrBlocks = append(cidrBlocks, "192.168.0.0/16") // Different networks
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		b.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	ip := net.ParseIP("192.168.1.100")
	if ip == nil {
		b.Fatalf("Invalid IP address")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.IsContained(ip)
	}
}
