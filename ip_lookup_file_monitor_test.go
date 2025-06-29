package traefik_geoblock

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"
)

// TestIpLookupFileMonitor_BasicDirectoryMonitoring tests basic directory monitoring functionality
func TestIpLookupFileMonitor_BasicDirectoryMonitoring(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("EmptyDirectory", func(t *testing.T) {
		tempDir := t.TempDir()

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Should work with empty directory
		contained, _, err := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if err != nil {
			t.Errorf("Lookup failed: %v", err)
		}
		if contained {
			t.Errorf("Expected no match in empty directory")
		}
	})

	t.Run("SingleFileInDirectory", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a single IP blocks file
		blockFile := filepath.Join(tempDir, "blocks.txt")
		blocks := []string{
			"192.168.0.0/16",
			"10.0.0.0/8",
			"172.16.0.0/12",
		}
		writeBlocksFile(t, blockFile, blocks)

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Test IP in range
		contained, prefixLen, err := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if err != nil {
			t.Errorf("Lookup failed: %v", err)
		}
		if !contained {
			t.Errorf("Expected IP 192.168.1.1 to be contained")
		}
		if prefixLen != 16 {
			t.Errorf("Expected prefix length 16, got %d", prefixLen)
		}

		// Test IP not in range
		contained, _, err = monitor.IsContained(net.ParseIP("8.8.8.8"))
		if err != nil {
			t.Errorf("Lookup failed: %v", err)
		}
		if contained {
			t.Errorf("Expected IP 8.8.8.8 to not be contained")
		}
	})

	t.Run("MultipleFilesInDirectory", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create multiple files with different IP blocks
		writeBlocksFile(t, filepath.Join(tempDir, "internal.txt"), []string{
			"192.168.0.0/16",
			"10.0.0.0/8",
		})
		writeBlocksFile(t, filepath.Join(tempDir, "cloud.txt"), []string{
			"172.16.0.0/12",
			"203.0.113.0/24",
		})
		writeBlocksFile(t, filepath.Join(tempDir, "public.txt"), []string{
			"8.8.8.0/24",
			"1.1.1.0/24",
		})

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		testCases := []struct {
			ip       string
			expected bool
		}{
			{"192.168.1.1", true}, // internal.txt
			{"10.0.0.1", true},    // internal.txt
			{"172.16.0.1", true},  // cloud.txt
			{"203.0.113.1", true}, // cloud.txt
			{"8.8.8.8", true},     // public.txt
			{"1.1.1.1", true},     // public.txt
			{"9.9.9.9", false},    // not in any file
		}

		for _, tc := range testCases {
			contained, _, err := monitor.IsContained(net.ParseIP(tc.ip))
			if err != nil {
				t.Errorf("Lookup failed for %s: %v", tc.ip, err)
			}
			if contained != tc.expected {
				t.Errorf("IP %s: expected %v, got %v", tc.ip, tc.expected, contained)
			}
		}
	})
}

// TestIpLookupFileMonitor_StaticBlocksAndDirectory tests combination of static blocks and directory
func TestIpLookupFileMonitor_StaticBlocksAndDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tempDir := t.TempDir()

	// Create directory file
	writeBlocksFile(t, filepath.Join(tempDir, "blocks.txt"), []string{
		"192.168.0.0/16",
	})

	// Static blocks
	staticBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
	}

	monitor, err := NewIpLookupFileMonitor(staticBlocks, tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	testCases := []struct {
		ip       string
		expected bool
		source   string
	}{
		{"10.0.0.1", true, "static"},
		{"172.16.0.1", true, "static"},
		{"192.168.1.1", true, "directory"},
		{"8.8.8.8", false, "none"},
	}

	for _, tc := range testCases {
		contained, _, err := monitor.IsContained(net.ParseIP(tc.ip))
		if err != nil {
			t.Errorf("Lookup failed for %s: %v", tc.ip, err)
		}
		if contained != tc.expected {
			t.Errorf("IP %s (%s): expected %v, got %v", tc.ip, tc.source, tc.expected, contained)
		}
	}
}

// TestIpLookupFileMonitor_SharedMonitors tests that multiple plugins share monitors correctly
func TestIpLookupFileMonitor_SharedMonitors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Clean up any existing monitors
	CleanupMonitors()

	tempDir := t.TempDir()
	writeBlocksFile(t, filepath.Join(tempDir, "blocks.txt"), []string{
		"192.168.0.0/16",
	})

	staticBlocks := []string{"172.16.0.0/12"}

	t.Run("SameConfiguration", func(t *testing.T) {
		// Create multiple monitors with identical configuration
		monitors := make([]*sharedIpLookupMonitor, 5)
		for i := 0; i < 5; i++ {
			var err error
			monitors[i], err = NewIpLookupFileMonitor(staticBlocks, tempDir, logger)
			if err != nil {
				t.Fatalf("Failed to create monitor %d: %v", i, err)
			}
		}

		// All monitors should return the same results
		testIP := net.ParseIP("192.168.1.1")
		for i, monitor := range monitors {
			contained, prefixLen, err := monitor.IsContained(testIP)
			if err != nil {
				t.Errorf("Monitor %d lookup failed: %v", i, err)
			}
			if !contained {
				t.Errorf("Monitor %d: expected IP to be contained", i)
			}
			if prefixLen != 16 {
				t.Errorf("Monitor %d: expected prefix length 16, got %d", i, prefixLen)
			}
		}

		// Verify shared monitors are actually sharing
		count := GetSharedMonitorCount()
		if count == 0 {
			t.Errorf("Expected shared monitors to exist")
		}

		t.Logf("Created %d monitor instances, sharing %d underlying monitors", len(monitors), count)
	})

	t.Run("DifferentConfigurations", func(t *testing.T) {
		// Clean up previous test
		CleanupMonitors()

		// Create monitors with different configurations
		monitor1, err := NewIpLookupFileMonitor([]string{"172.16.0.0/12"}, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor1: %v", err)
		}

		monitor2, err := NewIpLookupFileMonitor([]string{"172.16.0.0/12", "10.0.0.0/24"}, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor2: %v", err)
		}

		_, err = NewIpLookupFileMonitor([]string{"172.16.0.0/12"}, tempDir+"_different", logger)
		if err != nil {
			// This should fail because directory doesn't exist, which is expected
			t.Logf("Monitor3 failed as expected: %v", err)
		}

		// Verify they work independently
		testIP := net.ParseIP("10.0.0.1")

		contained1, _, _ := monitor1.IsContained(testIP)
		contained2, _, _ := monitor2.IsContained(testIP)

		// monitor1 should not contain 10.0.0.1 (only has 172.16.0.0/12 + directory 192.168.0.0/16)
		// monitor2 should contain 10.0.0.1 (has static 10.0.0.0/24)
		if contained1 {
			t.Errorf("Monitor1 should not contain 10.0.0.1")
		}
		if !contained2 {
			t.Errorf("Monitor2 should contain 10.0.0.1")
		}

		count := GetSharedMonitorCount()
		t.Logf("Different configurations created %d shared monitors", count)
	})
}

// TestIpLookupFileMonitor_ConcurrentAccess tests concurrent access to shared monitors
func TestIpLookupFileMonitor_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	CleanupMonitors()

	tempDir := t.TempDir()
	writeBlocksFile(t, filepath.Join(tempDir, "blocks.txt"), []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"203.0.113.0/24",
	})

	// Create shared monitor
	monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Test IPs
	testIPs := []string{
		"192.168.1.1",
		"10.0.0.1",
		"172.16.0.1",
		"203.0.113.1",
		"8.8.8.8", // should not match
	}

	// Run concurrent lookups
	const numGoroutines = 50
	const lookupsPerGoroutine = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < lookupsPerGoroutine; j++ {
				ip := testIPs[j%len(testIPs)]
				contained, _, err := monitor.IsContained(net.ParseIP(ip))
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, lookup %d: %v", goroutineID, j, err)
					return
				}

				// Verify expected results
				shouldContain := ip != "8.8.8.8"
				if contained != shouldContain {
					errors <- fmt.Errorf("goroutine %d, lookup %d: IP %s expected %v, got %v",
						goroutineID, j, ip, shouldContain, contained)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	t.Logf("Successfully completed %d concurrent lookups across %d goroutines",
		numGoroutines*lookupsPerGoroutine, numGoroutines)
}

// TestIpLookupFileMonitor_FileChanges tests file modification detection
func TestIpLookupFileMonitor_FileChanges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tempDir := t.TempDir()
	blockFile := filepath.Join(tempDir, "blocks.txt")

	// Create initial file
	writeBlocksFile(t, blockFile, []string{"192.168.0.0/16"})

	monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Verify initial state
	contained, _, err := monitor.IsContained(net.ParseIP("192.168.1.1"))
	if err != nil {
		t.Fatalf("Initial lookup failed: %v", err)
	}
	if !contained {
		t.Fatalf("Expected initial IP to be contained")
	}

	contained, _, err = monitor.IsContained(net.ParseIP("10.0.0.1"))
	if err != nil {
		t.Fatalf("Initial lookup failed: %v", err)
	}
	if contained {
		t.Fatalf("Expected 10.0.0.1 to not be contained initially")
	}

	// Modify file to add new block
	// Add a small delay to ensure file modification time changes
	time.Sleep(100 * time.Millisecond)
	writeBlocksFile(t, blockFile, []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
	})

	// Trigger reload manually since we don't want to wait for the ticker
	// (In real usage, this would happen automatically via file monitoring)
	err = monitor.checkAndReload()
	if err != nil {
		t.Fatalf("Failed to reload: %v", err)
	}

	// Verify new block is now included
	contained, _, err = monitor.IsContained(net.ParseIP("10.0.0.1"))
	if err != nil {
		t.Fatalf("Lookup after reload failed: %v", err)
	}
	if !contained {
		t.Errorf("Expected 10.0.0.1 to be contained after reload")
	}

	// Old block should still work
	contained, _, err = monitor.IsContained(net.ParseIP("192.168.1.1"))
	if err != nil {
		t.Fatalf("Lookup after reload failed: %v", err)
	}
	if !contained {
		t.Errorf("Expected 192.168.1.1 to still be contained after reload")
	}
}

// TestIpLookupFileMonitor_ErrorHandling tests various error conditions
func TestIpLookupFileMonitor_ErrorHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("NonexistentDirectory", func(t *testing.T) {
		CleanupMonitors()

		monitor, err := NewIpLookupFileMonitor(nil, "/nonexistent/directory", logger)
		if err != nil {
			t.Fatalf("Monitor creation should not fail for nonexistent directory: %v", err)
		}

		// Should work but find no matches
		contained, _, err := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if err != nil {
			t.Errorf("Lookup should not fail: %v", err)
		}
		if contained {
			t.Errorf("Should not find matches in nonexistent directory")
		}
	})

	t.Run("InvalidCIDRBlocks", func(t *testing.T) {
		CleanupMonitors()

		tempDir := t.TempDir()
		blockFile := filepath.Join(tempDir, "invalid.txt")

		// Create file with invalid CIDR blocks
		content := `# Valid block
192.168.0.0/16
# Invalid blocks below
invalid-cidr
192.168.1.0/33
not.an.ip/24
completely-malformed-entry
300.400.500.600/8
192.168.1.0/99
1.2.3/24
# Another valid block
10.0.0.0/8
`
		err := os.WriteFile(blockFile, []byte(content), 0600)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Monitor creation should not fail with invalid CIDRs: %v", err)
		}

		// Should still work with valid blocks
		contained, _, err := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if err != nil {
			t.Errorf("Lookup should not fail: %v", err)
		}
		if !contained {
			t.Errorf("Valid CIDR should still work")
		}

		contained, _, err = monitor.IsContained(net.ParseIP("10.0.0.1"))
		if err != nil {
			t.Errorf("Lookup should not fail: %v", err)
		}
		if !contained {
			t.Errorf("Valid CIDR should still work")
		}

		// Verify invalid entries are not blocking anything (test with valid IPs)
		invalidIPs := []string{"1.2.3.4", "5.6.7.8", "1.2.3.255"}
		for _, ip := range invalidIPs {
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil {
				t.Errorf("Test IP %s should be valid", ip)
				continue
			}
			contained, _, err := monitor.IsContained(parsedIP)
			if err != nil {
				t.Errorf("Lookup should not fail for %s: %v", ip, err)
			}
			if contained {
				t.Errorf("Invalid CIDR entries should not block IP %s", ip)
			}
		}
	})

	t.Run("MalformedFileHandling", func(t *testing.T) {
		CleanupMonitors()

		tempDir := t.TempDir()

		// Create multiple files, some with malformed content
		writeBlocksFile(t, filepath.Join(tempDir, "good.txt"), []string{
			"192.168.0.0/16",
			"10.0.0.0/8",
		})

		// File with mixed valid and invalid entries
		mixedFile := filepath.Join(tempDir, "mixed.txt")
		mixedContent := `# Mixed file with good and bad entries
172.16.0.0/12
invalid-line-here
203.0.113.0/24
another-invalid-entry
not.an.ip/subnet
8.8.8.0/24
# End of file
`
		err := os.WriteFile(mixedFile, []byte(mixedContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create mixed file: %v", err)
		}

		// File with only invalid entries
		badFile := filepath.Join(tempDir, "bad.txt")
		badContent := `# File with only invalid entries
totally-invalid
256.256.256.256/8
not-an-ip-at-all
192.168.1.0/999
`
		err = os.WriteFile(badFile, []byte(badContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create bad file: %v", err)
		}

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Monitor creation should not fail with malformed files: %v", err)
		}

		// Test that valid entries from good files work
		goodIPs := map[string]bool{
			"192.168.1.1": true, // good.txt
			"10.0.0.1":    true, // good.txt
			"172.16.0.1":  true, // mixed.txt
			"203.0.113.1": true, // mixed.txt
			"8.8.8.8":     true, // mixed.txt
		}

		for ip, expected := range goodIPs {
			contained, _, err := monitor.IsContained(net.ParseIP(ip))
			if err != nil {
				t.Errorf("Lookup failed for %s: %v", ip, err)
			}
			if contained != expected {
				t.Errorf("IP %s: expected %v, got %v", ip, expected, contained)
			}
		}

		// Test that invalid entries don't interfere (test with valid IPs not in our ranges)
		invalidIPs := []string{"1.2.3.4", "9.9.9.9", "5.6.7.8"}
		for _, ip := range invalidIPs {
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil {
				t.Errorf("Test IP %s should be valid", ip)
				continue
			}
			contained, _, err := monitor.IsContained(parsedIP)
			if err != nil {
				t.Errorf("Lookup should not fail for %s: %v", ip, err)
			}
			if contained {
				t.Errorf("Invalid CIDR entries should not affect IP %s", ip)
			}
		}
	})

	t.Run("NonTxtFiles", func(t *testing.T) {
		// Clean up monitors to avoid interference from other tests
		CleanupMonitors()

		tempDir := t.TempDir()

		// Create various file types
		writeBlocksFile(t, filepath.Join(tempDir, "blocks.txt"), []string{"192.168.0.0/16"})

		// These should be ignored
		if err := os.WriteFile(filepath.Join(tempDir, "readme.md"), []byte("# Readme\n10.0.0.0/8"), 0600); err != nil {
			t.Fatalf("Failed to create readme.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(`{"cidr": "172.16.0.0/12"}`), 0600); err != nil {
			t.Fatalf("Failed to create config.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, "uppercase.TXT"), []byte("203.0.113.0/24"), 0600); err != nil {
			t.Fatalf("Failed to create uppercase.TXT: %v", err)
		}

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Only .txt files should be processed
		contained, _, _ := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if !contained {
			t.Errorf("Expected blocks.txt to be processed")
		}

		contained, _, _ = monitor.IsContained(net.ParseIP("203.0.113.1"))
		if !contained {
			t.Errorf("Expected uppercase.TXT (uppercase) to be processed")
		}

		contained, _, _ = monitor.IsContained(net.ParseIP("10.0.0.1"))
		if contained {
			t.Errorf("Expected readme.md to be ignored")
		}

		contained, _, _ = monitor.IsContained(net.ParseIP("172.16.0.1"))
		if contained {
			t.Errorf("Expected config.json to be ignored")
		}
	})

	t.Run("NilIPHandling", func(t *testing.T) {
		CleanupMonitors()

		tempDir := t.TempDir()
		writeBlocksFile(t, filepath.Join(tempDir, "blocks.txt"), []string{"192.168.0.0/16"})

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Test with nil IP (should return error, not panic)
		contained, prefixLen, err := monitor.IsContained(nil)
		if err == nil {
			t.Errorf("Expected error when passing nil IP")
		}
		if contained {
			t.Errorf("Expected nil IP to not be contained")
		}
		if prefixLen != 0 {
			t.Errorf("Expected prefix length 0 for nil IP, got %d", prefixLen)
		}

		// Test with malformed IP string parsing (simulate real-world scenario)
		invalidIPStrings := []string{"300.400.500.600", "not.an.ip", "192.168.1.0/24"}
		for _, ipStr := range invalidIPStrings {
			parsedIP := net.ParseIP(ipStr)
			if parsedIP != nil {
				t.Errorf("Expected %s to be unparseable by net.ParseIP", ipStr)
				continue
			}

			// Verify calling with nil doesn't panic
			contained, prefixLen, err := monitor.IsContained(parsedIP)
			if err == nil {
				t.Errorf("Expected error when passing nil IP parsed from %s", ipStr)
			}
			if contained {
				t.Errorf("Expected malformed IP %s to not be contained", ipStr)
			}
			if prefixLen != 0 {
				t.Errorf("Expected prefix length 0 for malformed IP %s, got %d", ipStr, prefixLen)
			}
		}
	})
}

// TestIpLookupFileMonitor_PluginIntegration tests integration with plugin system
func TestIpLookupFileMonitor_PluginIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test IP blocks
	writeBlocksFile(t, filepath.Join(tempDir, "allowed.txt"), []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
	})

	// Create multiple plugin instances that would share the same directory
	configs := []*Config{
		{
			Enabled:              true,
			DatabaseFilePath:     "./IP2LOCATION-LITE-DB1.IPV6.BIN",
			AllowedIPBlocksDir:   tempDir,
			DisallowedStatusCode: 403,
			IPHeaders:            []string{"x-forwarded-for"},
		},
		{
			Enabled:              true,
			DatabaseFilePath:     "./IP2LOCATION-LITE-DB1.IPV6.BIN",
			AllowedIPBlocksDir:   tempDir, // Same directory
			DisallowedStatusCode: 403,
			IPHeaders:            []string{"x-forwarded-for"},
		},
		{
			Enabled:              true,
			DatabaseFilePath:     "./IP2LOCATION-LITE-DB1.IPV6.BIN",
			BlockedIPBlocksDir:   tempDir, // Different usage of same directory
			DisallowedStatusCode: 403,
			IPHeaders:            []string{"x-forwarded-for"},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	CleanupMonitors()

	// Create multiple plugin instances
	plugins := make([]http.Handler, len(configs))
	for i, config := range configs {
		ctx := context.Background()
		plugin, err := New(ctx, nextHandler, config, fmt.Sprintf("test-plugin-%d", i))
		if err != nil {
			t.Fatalf("Failed to create plugin %d: %v", i, err)
		}
		plugins[i] = plugin
	}

	// Verify shared monitors are created
	count := GetSharedMonitorCount()
	t.Logf("Created %d plugin instances sharing %d monitors", len(plugins), count)

	if count == 0 {
		t.Errorf("Expected some shared monitors to be created")
	}

	// Test that all plugins work correctly
	for i, plugin := range plugins {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1") // Should be allowed/blocked based on config

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		// All should process the request (specific behavior depends on config)
		t.Logf("Plugin %d response status: %d", i, rr.Code)
	}
}

// TestIpLookupFileMonitor_DynamicFileOperations tests adding and removing files dynamically
func TestIpLookupFileMonitor_DynamicFileOperations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	CleanupMonitors()

	t.Run("FileAddition", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create initial file with some blocks
		writeBlocksFile(t, filepath.Join(tempDir, "initial.txt"), []string{
			"192.168.0.0/16",
			"10.0.0.0/8",
		})

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Verify initial state - these IPs should be blocked
		contained, _, err := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if err != nil {
			t.Fatalf("Initial lookup failed: %v", err)
		}
		if !contained {
			t.Errorf("Expected 192.168.1.1 to be contained initially")
		}

		// Verify AWS IP is not blocked initially
		awsIP := net.ParseIP("172.16.0.1")
		contained, _, err = monitor.IsContained(awsIP)
		if err != nil {
			t.Fatalf("AWS IP lookup failed: %v", err)
		}
		if contained {
			t.Errorf("Expected AWS IP 172.16.0.1 to NOT be contained initially")
		}

		// Add a new file with AWS blocks
		writeBlocksFile(t, filepath.Join(tempDir, "aws.txt"), []string{
			"172.16.0.0/12",
			"203.0.113.0/24",
		})

		// Force refresh to pick up the new file
		err = monitor.ForceRefresh()
		if err != nil {
			t.Fatalf("Failed to force refresh: %v", err)
		}

		// Now AWS IP should be blocked
		contained, _, err = monitor.IsContained(awsIP)
		if err != nil {
			t.Fatalf("AWS IP lookup after addition failed: %v", err)
		}
		if !contained {
			t.Errorf("Expected AWS IP 172.16.0.1 to be contained after adding aws.txt")
		}

		// Original IPs should still be blocked
		contained, _, err = monitor.IsContained(net.ParseIP("192.168.1.1"))
		if err != nil {
			t.Fatalf("Original IP lookup after addition failed: %v", err)
		}
		if !contained {
			t.Errorf("Expected original IP 192.168.1.1 to still be contained")
		}

		// Test another IP from the new file
		contained, _, err = monitor.IsContained(net.ParseIP("203.0.113.5"))
		if err != nil {
			t.Fatalf("New IP lookup failed: %v", err)
		}
		if !contained {
			t.Errorf("Expected new IP 203.0.113.5 to be contained after adding aws.txt")
		}
	})

	t.Run("FileRemoval", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create multiple files
		writeBlocksFile(t, filepath.Join(tempDir, "internal.txt"), []string{
			"192.168.0.0/16",
			"10.0.0.0/8",
		})

		awsFile := filepath.Join(tempDir, "aws.txt")
		writeBlocksFile(t, awsFile, []string{
			"172.16.0.0/12",
			"203.0.113.0/24",
		})

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Verify both files are loaded
		testCases := []struct {
			ip     string
			source string
		}{
			{"192.168.1.1", "internal.txt"},
			{"10.0.0.1", "internal.txt"},
			{"172.16.0.1", "aws.txt"},
			{"203.0.113.1", "aws.txt"},
		}

		for _, tc := range testCases {
			contained, _, err := monitor.IsContained(net.ParseIP(tc.ip))
			if err != nil {
				t.Errorf("Initial lookup failed for %s: %v", tc.ip, err)
			}
			if !contained {
				t.Errorf("Expected %s to be contained initially (from %s)", tc.ip, tc.source)
			}
		}

		// Remove the AWS file
		err = os.Remove(awsFile)
		if err != nil {
			t.Fatalf("Failed to remove AWS file: %v", err)
		}

		// Force refresh to pick up the file removal
		err = monitor.ForceRefresh()
		if err != nil {
			t.Fatalf("Failed to force refresh after removal: %v", err)
		}

		// AWS IPs should no longer be blocked
		awsIPs := []string{"172.16.0.1", "203.0.113.1"}
		for _, ip := range awsIPs {
			contained, _, err := monitor.IsContained(net.ParseIP(ip))
			if err != nil {
				t.Errorf("AWS IP lookup after removal failed for %s: %v", ip, err)
			}
			if contained {
				t.Errorf("Expected AWS IP %s to NOT be contained after removing aws.txt", ip)
			}
		}

		// Internal IPs should still be blocked
		internalIPs := []string{"192.168.1.1", "10.0.0.1"}
		for _, ip := range internalIPs {
			contained, _, err := monitor.IsContained(net.ParseIP(ip))
			if err != nil {
				t.Errorf("Internal IP lookup after removal failed for %s: %v", ip, err)
			}
			if !contained {
				t.Errorf("Expected internal IP %s to still be contained after removing aws.txt", ip)
			}
		}
	})

	t.Run("FileAdditionAndRemovalCombined", func(t *testing.T) {
		tempDir := t.TempDir()

		// Start with one file
		writeBlocksFile(t, filepath.Join(tempDir, "base.txt"), []string{
			"192.168.0.0/16",
		})

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Verify initial state
		contained, _, _ := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if !contained {
			t.Errorf("Expected 192.168.1.1 to be contained initially")
		}

		// Add two new files
		cloudFile := filepath.Join(tempDir, "cloud.txt")
		writeBlocksFile(t, cloudFile, []string{
			"172.16.0.0/12",
		})

		writeBlocksFile(t, filepath.Join(tempDir, "public.txt"), []string{
			"8.8.8.0/24",
		})

		// Force refresh
		err = monitor.ForceRefresh()
		if err != nil {
			t.Fatalf("Failed to force refresh after addition: %v", err)
		}

		// All should be blocked now
		testIPs := map[string]bool{
			"192.168.1.1": true,
			"172.16.0.1":  true,
			"8.8.8.8":     true,
			"203.0.113.1": false, // Not in any file
		}

		for ip, expected := range testIPs {
			contained, _, _ := monitor.IsContained(net.ParseIP(ip))
			if contained != expected {
				t.Errorf("After addition: IP %s expected %v, got %v", ip, expected, contained)
			}
		}

		// Now remove the cloud file
		err = os.Remove(cloudFile)
		if err != nil {
			t.Fatalf("Failed to remove cloud file: %v", err)
		}

		// Force refresh
		err = monitor.ForceRefresh()
		if err != nil {
			t.Fatalf("Failed to force refresh after removal: %v", err)
		}

		// Cloud IPs should no longer be blocked
		finalTestIPs := map[string]bool{
			"192.168.1.1": true,  // base.txt
			"172.16.0.1":  false, // cloud.txt removed
			"8.8.8.8":     true,  // public.txt
			"203.0.113.1": false, // Not in any file
		}

		for ip, expected := range finalTestIPs {
			contained, _, _ := monitor.IsContained(net.ParseIP(ip))
			if contained != expected {
				t.Errorf("After removal: IP %s expected %v, got %v", ip, expected, contained)
			}
		}
	})

	t.Run("FileModification", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a file
		modifyFile := filepath.Join(tempDir, "modify.txt")
		writeBlocksFile(t, modifyFile, []string{
			"192.168.0.0/16",
		})

		monitor, err := NewIpLookupFileMonitor(nil, tempDir, logger)
		if err != nil {
			t.Fatalf("Failed to create monitor: %v", err)
		}

		// Verify initial content
		contained, _, _ := monitor.IsContained(net.ParseIP("192.168.1.1"))
		if !contained {
			t.Errorf("Expected 192.168.1.1 to be contained initially")
		}

		contained, _, _ = monitor.IsContained(net.ParseIP("10.0.0.1"))
		if contained {
			t.Errorf("Expected 10.0.0.1 to NOT be contained initially")
		}

		// Sleep briefly to ensure file modification time is different
		time.Sleep(10 * time.Millisecond)

		// Modify the file to add new blocks
		writeBlocksFile(t, modifyFile, []string{
			"192.168.0.0/16",
			"10.0.0.0/8",
			"172.16.0.0/12",
		})

		// Force refresh
		err = monitor.ForceRefresh()
		if err != nil {
			t.Fatalf("Failed to force refresh after modification: %v", err)
		}

		// New blocks should now be active
		newIPs := []string{"10.0.0.1", "172.16.0.1"}
		for _, ip := range newIPs {
			contained, _, _ := monitor.IsContained(net.ParseIP(ip))
			if !contained {
				t.Errorf("Expected %s to be contained after file modification", ip)
			}
		}

		// Original block should still work
		contained, _, _ = monitor.IsContained(net.ParseIP("192.168.1.1"))
		if !contained {
			t.Errorf("Expected 192.168.1.1 to still be contained after modification")
		}
	})
}

// Helper function to write CIDR blocks to a file
func writeBlocksFile(t *testing.T, filename string, blocks []string) {
	content := strings.Join(blocks, "\n")
	err := os.WriteFile(filename, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to write blocks file: %v", err)
	}
}
