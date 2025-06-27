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

func TestIpLookupHelper_PrefixLengthAccuracy(t *testing.T) {
	tests := []struct {
		name       string
		cidrBlocks []string
		testCases  []struct {
			ip             string
			expectedPrefix int
			shouldMatch    bool
		}
	}{
		{
			name: "IPv4 Various Prefix Lengths",
			cidrBlocks: []string{
				"0.0.0.0/0",        // All IPv4 - /0
				"10.0.0.0/8",       // Class A - /8
				"192.168.0.0/16",   // Class C network - /16
				"203.0.113.0/24",   // Subnet - /24
				"198.51.100.50/32", // Single IP - /32
			},
			testCases: []struct {
				ip             string
				expectedPrefix int
				shouldMatch    bool
			}{
				// Should match most specific (longest prefix)
				{"10.5.10.20", 8, true},     // Matches /8, not /0
				{"192.168.5.10", 16, true},  // Matches /16, not /0
				{"203.0.113.100", 24, true}, // Matches /24, not /0
				{"198.51.100.50", 32, true}, // Matches /32 (exact), not /0
				{"8.8.8.8", 0, true},        // Only matches /0 (catch-all)
				{"1.2.3.4", 0, true},        // Only matches /0 (catch-all)
			},
		},
		{
			name: "IPv6 Various Prefix Lengths",
			cidrBlocks: []string{
				"::/0",                   // All IPv6 - /0
				"2001:db8::/32",          // Large network - /32
				"2001:db8:85a3::/48",     // Subnet - /48
				"2001:db8:85a3:8d3::/64", // Host subnet - /64
				"::1/128",                // Single IP - /128
			},
			testCases: []struct {
				ip             string
				expectedPrefix int
				shouldMatch    bool
			}{
				// Should match most specific (longest prefix)
				{"2001:db8:1234:5678::1", 32, true},     // Matches /32, not /0
				{"2001:db8:85a3:1234::1", 48, true},     // Matches /48, not /32 or /0
				{"2001:db8:85a3:8d3:1234::1", 64, true}, // Matches /64, not /48, /32, or /0
				{"::1", 128, true},                      // Matches /128 (exact), not /0
				{"2001:db9::1", 0, true},                // Only matches /0 (catch-all)
			},
		},
		{
			name: "Overlapping IPv4 Networks",
			cidrBlocks: []string{
				"192.168.0.0/16",   // Large network
				"192.168.1.0/24",   // Subnet within above
				"192.168.1.100/30", // Even smaller subnet (4 IPs: .100-.103)
				"192.168.1.101/32", // Single IP within above
			},
			testCases: []struct {
				ip             string
				expectedPrefix int
				shouldMatch    bool
			}{
				{"192.168.1.101", 32, true}, // Most specific: /32
				{"192.168.1.100", 30, true}, // Most specific: /30 (not /24 or /16)
				{"192.168.1.102", 30, true}, // Most specific: /30
				{"192.168.1.103", 30, true}, // Most specific: /30
				{"192.168.1.104", 24, true}, // Most specific: /24 (not in /30)
				{"192.168.1.50", 24, true},  // Most specific: /24
				{"192.168.2.1", 16, true},   // Most specific: /16
				{"192.169.1.1", 0, false},   // No match
			},
		},
		{
			name: "Edge Case Prefixes",
			cidrBlocks: []string{
				"127.0.0.0/8",        // Loopback network
				"127.0.0.1/32",       // Specific loopback IP
				"255.255.255.255/32", // Broadcast address
				"224.0.0.0/4",        // Multicast range
			},
			testCases: []struct {
				ip             string
				expectedPrefix int
				shouldMatch    bool
			}{
				{"127.0.0.1", 32, true},       // Most specific match
				{"127.0.0.2", 8, true},        // Loopback network
				{"127.1.1.1", 8, true},        // Loopback network
				{"255.255.255.255", 32, true}, // Exact match
				{"224.0.0.1", 4, true},        // Multicast
				{"239.255.255.255", 4, true},  // Multicast (end of range)
				{"240.0.0.1", 0, false},       // No match
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper, err := NewIpLookupHelper(tt.cidrBlocks)
			if err != nil {
				t.Fatalf("Failed to create IpLookupHelper: %v", err)
			}

			for _, tc := range tt.testCases {
				t.Run("IP_"+tc.ip, func(t *testing.T) {
					ip := net.ParseIP(tc.ip)
					if ip == nil {
						t.Fatalf("Invalid IP address: %s", tc.ip)
					}

					found, prefixLen, err := helper.IsContained(ip)
					if err != nil {
						t.Errorf("IsContained returned error: %v", err)
					}

					if found != tc.shouldMatch {
						t.Errorf("IsContained(%s) = %v, want %v", tc.ip, found, tc.shouldMatch)
					}

					if found && prefixLen != tc.expectedPrefix {
						t.Errorf("IsContained(%s) returned prefix %d, want %d (IP should match most specific CIDR)",
							tc.ip, prefixLen, tc.expectedPrefix)
					}

					if !found && tc.shouldMatch {
						t.Errorf("IsContained(%s) should have matched with prefix %d", tc.ip, tc.expectedPrefix)
					}
				})
			}
		})
	}
}

func TestIpLookupHelper_PrefixLengthConsistency(t *testing.T) {
	// Test that the same CIDR block always returns the same prefix length
	cidrBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"203.0.113.0/24",
		"198.51.100.1/32",
		"2001:db8::/32",
		"fe80::/10",
		"::1/128",
	}

	helper, err := NewIpLookupHelper(cidrBlocks)
	if err != nil {
		t.Fatalf("Failed to create IpLookupHelper: %v", err)
	}

	testCases := []struct {
		ip             string
		expectedPrefix int
	}{
		{"10.1.2.3", 8},
		{"172.16.0.1", 12},
		{"192.168.1.1", 16},
		{"203.0.113.50", 24},
		{"198.51.100.1", 32},
		{"2001:db8::1", 32},
		{"fe80::1", 10},
		{"::1", 128},
	}

	// Test multiple times to ensure consistency
	for i := 0; i < 10; i++ {
		for _, tc := range testCases {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("Invalid IP address: %s", tc.ip)
			}

			found, prefixLen, err := helper.IsContained(ip)
			if err != nil {
				t.Errorf("IsContained returned error on iteration %d: %v", i, err)
			}

			if !found {
				t.Errorf("IsContained(%s) should have matched on iteration %d", tc.ip, i)
				continue
			}

			if prefixLen != tc.expectedPrefix {
				t.Errorf("Iteration %d: IsContained(%s) returned inconsistent prefix %d, want %d",
					i, tc.ip, prefixLen, tc.expectedPrefix)
			}
		}
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
		_, _, err := helper.IsContained(ip)
		if err != nil {
			b.Fatalf("IsContained returned error: %v", err)
		}
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
		_, _, err := helper.IsContained(ip)
		if err != nil {
			b.Fatalf("IsContained returned error: %v", err)
		}
	}
}
