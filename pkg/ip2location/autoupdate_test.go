package ip2location

import (
	"bytes"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ip2loc "github.com/ip2location/ip2location-go/v9"
)

func TestPlugin_DatabaseDownloadAndUpdate(t *testing.T) {
	// Create a temporary directory for testing
	err := os.MkdirAll("./testdata/autoupdate", 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll("./testdata/autoupdate")

	tests := []struct {
		name        string
		config      *DatabaseConfig
		setupFiles  func(t *testing.T, dir string) // Setup test files if needed
		wantErr     bool
		errContains string
		checkResult func(t *testing.T, dir string) // Verify results if needed
	}{
		{
			name: "find latest database",
			config: &DatabaseConfig{
				DatabaseAutoUpdate:     true,
				DatabaseAutoUpdateDir:  "./testdata/autoupdate",
				DatabaseAutoUpdateCode: "DB1",
			},
			setupFiles: func(t *testing.T, dir string) {
				// Create some test database files with different dates
				files := []string{
					"20230101_IP2LOCATION-LITE-DB1.IPV6.BIN",
					"20230201_IP2LOCATION-LITE-DB1.IPV6.BIN",
					"20230301_IP2LOCATION-LITE-DB1.IPV6.BIN",
				}
				for _, f := range files {
					path := filepath.Join(dir, f)
					if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
						t.Fatalf("Failed to create test file %s: %v", f, err)
					}
				}
			},
			checkResult: func(t *testing.T, dir string) {
				latestDB, err := findLatestDatabase(dir, "DB1")
				if err != nil {
					t.Errorf("expected to find updated database file, but got error: %v", err)
					return
				}
				if latestDB == "" {
					t.Errorf("expected updated database file, but found none in %s", dir)
					return
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files if needed
			if tt.setupFiles != nil {
				tt.setupFiles(t, tt.config.DatabaseAutoUpdateDir)
			}

			// Create logger for testing
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})).With("plugin", "test")

			// Test database update
			err := downloadAndUpdateDatabase(tt.config, logger)

			// Check error conditions
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, but got none")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			// Check results if needed
			if tt.checkResult != nil {
				tt.checkResult(t, tt.config.DatabaseAutoUpdateDir)
			}
		})
	}
}

func TestUpdateIfNeeded(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		dbPath       string
		setupFunc    func(string)             // Optional setup function
		validateFunc func(*testing.T, string) // Optional validation function
		wantErr      bool
		errContains  string
	}{
		{
			name:   "empty path triggers successful download",
			dbPath: "",
			validateFunc: func(t *testing.T, dir string) {
				// Find the downloaded file
				files, err := filepath.Glob(filepath.Join(dir, "*_IP2LOCATION-LITE-DB1.IPV6.BIN"))
				if err != nil || len(files) == 0 {
					t.Errorf("expected downloaded database file, but found none in %s", dir)
					return
				}

				// Verify the downloaded database is valid by trying to open it
				db, err := ip2loc.OpenDB(files[0])
				if err != nil {
					t.Errorf("downloaded database is not valid: %v", err)
					return
				}
				defer db.Close()

				// Test a known IP lookup
				record, err := db.Get_country_short("8.8.8.8")
				if err != nil {
					t.Errorf("failed to lookup test IP: %v", err)
					return
				}
				if record.Country_short != "US" {
					t.Errorf("expected country US for 8.8.8.8, got %s", record.Country_short)
				}
			},
		},
		{
			name:   "recent database needs no update",
			dbPath: filepath.Join(tmpDir, time.Now().Format("20060102")+"_IP2LOCATION-LITE-DB1.IPV6.BIN"),
			setupFunc: func(path string) {
				// Copy an existing valid database to this location
				source := filepath.Join("..", "..", "IP2LOCATION-LITE-DB1.IPV6.BIN")
				content, err := os.ReadFile(source)
				if err == nil {
					_ = os.WriteFile(path, content, 0600)
				}
			},
		},
		{
			name:   "old database triggers update",
			dbPath: filepath.Join(tmpDir, time.Now().AddDate(0, -2, 0).Format("20060102")+"_IP2LOCATION-LITE-DB1.IPV6.BIN"),
			setupFunc: func(path string) {
				_ = os.WriteFile(path, []byte("old data"), 0600)
			},
			validateFunc: func(t *testing.T, dir string) {
				// Verify we got a new database file
				latest, err := findLatestDatabase(dir, "DB1")
				if err != nil || latest == "" {
					t.Errorf("expected updated database file, but found none in %s", dir)
					return
				}
				files := []string{latest}

				// Verify the new database is valid
				db, err := ip2loc.OpenDB(files[0])
				if err != nil {
					t.Errorf("updated database is not valid: %v", err)
					return
				}
				defer db.Close()
			},
		},
		{
			name:    "invalid filename format",
			dbPath:  filepath.Join(tmpDir, "invalid-filename.bin"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run setup if provided
			if tt.setupFunc != nil {
				tt.setupFunc(tt.dbPath)
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))

			cfg := &DatabaseConfig{
				DatabaseAutoUpdateDir:  tmpDir,
				DatabaseAutoUpdateCode: "DB1",
			}

			_ = os.RemoveAll(tmpDir)

			err := UpdateIfNeeded(tt.dbPath, true, logger, cfg)

			// Check error conditions
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, but got none")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Run validation if provided
			if tt.validateFunc != nil {
				tt.validateFunc(t, tmpDir)
			}
		})
	}
}

func TestFindLatestDatabase(t *testing.T) {
	// Create a temporary directory for testing
	testDir := t.TempDir()

	// Create some test database files with different dates
	files := []string{
		"20230101_IP2LOCATION-LITE-DB1.IPV6.BIN",
		"20230201_IP2LOCATION-LITE-DB1.IPV6.BIN",
		"20230301_IP2LOCATION-LITE-DB1.IPV6.BIN",
	}
	for _, f := range files {
		path := filepath.Join(testDir, f)
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f, err)
		}
	}

	// Test finding the latest database
	latest, err := findLatestDatabase(testDir, "DB1")
	if err != nil {
		t.Errorf("findLatestDatabase failed: %v", err)
	}
	if !strings.Contains(latest, "20230301") {
		t.Errorf("Expected latest database to be from March 2023, got %s", latest)
	}
}

func TestDownloadTwiceReturnsNoError(t *testing.T) {
	// This test verifies that calling downloadAndUpdateDatabase twice
	// (simulating a restart when IP2Location hasn't released a newer database)
	// does not return an error on the second call.
	// See: https://github.com/david-garcia-garcia/traefik-geoblock/issues/40

	targetDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("plugin", "test")

	cfg := &DatabaseConfig{
		DatabaseAutoUpdate:     true,
		DatabaseAutoUpdateDir:  targetDir,
		DatabaseAutoUpdateCode: "DB1",
	}

	// First download - should succeed and create the database file
	err := downloadAndUpdateDatabase(cfg, logger)
	if err != nil {
		t.Fatalf("first download failed: %v", err)
	}

	// Verify database was downloaded
	latestDB, err := findLatestDatabase(targetDir, "DB1")
	if err != nil {
		t.Fatalf("failed to find database after first download: %v", err)
	}
	if latestDB == "" {
		t.Fatal("no database file found after first download")
	}

	// Second download - capture log output to verify specific warning message
	// This simulates a container restart when IP2Location hasn't released a newer database
	var logBuffer bytes.Buffer
	loggerWithCapture := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("plugin", "test")

	err = downloadAndUpdateDatabase(cfg, loggerWithCapture)
	if err != nil {
		t.Errorf("second download should not return error when database already exists, got: %v", err)
	}

	// Verify the specific warning message was logged
	logOutput := logBuffer.String()
	expectedWarning := "the available IP2Location database is not newer than the one already available"
	if !strings.Contains(logOutput, expectedWarning) {
		t.Errorf("expected log to contain warning %q, got: %s", expectedWarning, logOutput)
	}

	// Verify only one database file exists (no duplicates created)
	files, err := filepath.Glob(filepath.Join(targetDir, "*_IP2LOCATION-LITE-DB1.IPV6.BIN"))
	if err != nil {
		t.Fatalf("failed to list database files: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected exactly 1 database file, got %d: %v", len(files), files)
	}
}

func TestDatabaseDirectoryIsCreatedAndDatabaseDownloaded(t *testing.T) {
	// Create a temporary base directory for testing
	tempBase := t.TempDir()
	targetDir := filepath.Join(tempBase, "new_directory")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("plugin", "test")

	cfg := &DatabaseConfig{
		DatabaseAutoUpdate:     true,
		DatabaseAutoUpdateDir:  targetDir,
		DatabaseAutoUpdateCode: "DB1",
	}

	// Verify directory doesn't exist before test
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Fatalf("directory should not exist before test: %v", err)
	}

	err := downloadAndUpdateDatabase(cfg, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Error("directory was not created")
	}

	// Find the latest database file
	latestDB, err := findLatestDatabase(targetDir, "DB1")
	if err != nil {
		t.Errorf("failed to find latest database: %v", err)
		return
	}
	if latestDB == "" {
		t.Error("no database file was downloaded")
		return
	}

	// Verify the downloaded database is valid
	db, err := ip2loc.OpenDB(latestDB)
	if err != nil {
		t.Errorf("downloaded database is not valid: %v", err)
		return
	}
	defer db.Close()

	// Test a known IP lookup to verify database contents
	record, err := db.Get_country_short("8.8.8.8")
	if err != nil {
		t.Errorf("failed to lookup test IP: %v", err)
		return
	}
	if record.Country_short != "US" {
		t.Errorf("expected country US for 8.8.8.8, got %s", record.Country_short)
	}
}

func TestIp2locationDownloadURL_TokenFileIsExactCode(t *testing.T) {
	const token = "test-token"
	oldZip := "IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP"

	cases := []struct {
		code string
		want string
	}{
		{code: "DB8BINIPV6", want: "DB8BINIPV6"},
		{code: "DB1LITEBINIPV6", want: "DB1LITEBINIPV6"},
		{code: "DB8", want: "DB8"},
		{code: "DB1", want: "DB1"},
	}

	for _, tc := range cases {
		got := ip2locationDownloadURL(token, tc.code)
		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("code %s: parse url: %v", tc.code, err)
		}
		file := u.Query().Get("file")
		if file != tc.want {
			t.Errorf("code %s: file=%q, want %q", tc.code, file, tc.want)
		}
		if strings.Contains(file, oldZip) || file == oldZip {
			t.Errorf("code %s: file=%q must not be the old ZIP filename", tc.code, file)
		}
	}

	if got := ip2locationDownloadURL("", "DB1"); got != liteDownloadURL {
		t.Errorf("empty token: got %q, want liteDownloadURL", got)
	}
	if got := ip2locationDownloadURL("", DefaultASNDatabaseCode); got != asnLiteDownloadURL {
		t.Errorf("empty token ASN IPv6: got %q, want asnLiteDownloadURL", got)
	}
	if got := ip2locationDownloadURL("", "DBASNLITEBIN"); got != asnLiteIPv4URL {
		t.Errorf("empty token ASN IPv4: got %q, want asnLiteIPv4URL", got)
	}
	got := ip2locationDownloadURL(token, DefaultASNDatabaseCode)
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("ASN token url: %v", err)
	}
	if file := u.Query().Get("file"); file != DefaultASNDatabaseCode {
		t.Errorf("ASN token file=%q, want %q", file, DefaultASNDatabaseCode)
	}
}

func TestFindLatestDatabase_ASNCode(t *testing.T) {
	testDir := t.TempDir()
	files := []string{
		"20230101_IP2LOCATION-LITE-DBASNLITEBINIPV6.IPV6.BIN",
		"20230301_IP2LOCATION-LITE-DBASNLITEBINIPV6.IPV6.BIN",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(testDir, f), []byte("test"), 0600); err != nil {
			t.Fatalf("create %s: %v", f, err)
		}
	}
	latest, err := findLatestDatabase(testDir, DefaultASNDatabaseCode)
	if err != nil {
		t.Fatalf("findLatestDatabase: %v", err)
	}
	if !strings.Contains(latest, "20230301") {
		t.Errorf("expected March 2023 ASN file, got %s", latest)
	}
}
