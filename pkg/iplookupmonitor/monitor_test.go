package iplookupmonitor

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestMonitor_BasicDirectoryMonitoring tests basic directory monitoring functionality
func TestMonitor_BasicDirectoryMonitoring(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("EmptyDirectory", func(t *testing.T) {
		tempDir := t.TempDir()

		monitor, err := New(nil, tempDir, logger)
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

		monitor, err := New(nil, tempDir, logger)
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

		monitor, err := New(nil, tempDir, logger)
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

// TestMonitor_StaticBlocksAndDirectory tests combination of static blocks and directory
func TestMonitor_StaticBlocksAndDirectory(t *testing.T) {
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

	monitor, err := New(staticBlocks, tempDir, logger)
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

// TestMonitor_ConcurrentAccess tests concurrent access to monitors
func TestMonitor_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tempDir := t.TempDir()
	writeBlocksFile(t, filepath.Join(tempDir, "blocks.txt"), []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"203.0.113.0/24",
	})

	// Create monitor
	monitor, err := New(nil, tempDir, logger)
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

// TestMonitor_ErrorHandling tests various error conditions
func TestMonitor_ErrorHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("NonexistentDirectory", func(t *testing.T) {
		monitor, err := New(nil, "/nonexistent/directory", logger)
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

		monitor, err := New(nil, tempDir, logger)
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
	})

	t.Run("NilIPHandling", func(t *testing.T) {
		tempDir := t.TempDir()
		writeBlocksFile(t, filepath.Join(tempDir, "blocks.txt"), []string{"192.168.0.0/16"})

		monitor, err := New(nil, tempDir, logger)
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
	})
}

func writeBlocksFile(t *testing.T, filename string, blocks []string) {
	content := strings.Join(blocks, "\n") + "\n"
	err := os.WriteFile(filename, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to write blocks file %s: %v", filename, err)
	}
}
