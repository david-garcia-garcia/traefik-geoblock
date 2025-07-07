package traefik_geoblock

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"log/slog"
)

//go:generate go run ./tools/dbdownload/main.go -o ./IP2LOCATION-LITE-DB1.IPV6.BIN

// Add this constant near the top of the file, after imports
const PrivateIpCountryAlias = "PRIVATE"

// Config defines the plugin configuration.
type Config struct {
	// Core settings
	Enabled          bool   // Enable/disable the plugin
	DatabaseFilePath string // Path to ip2location database file
	DefaultAllow     bool   // Default behavior when IP matches no rules
	AllowPrivate     bool   // Allow requests from private/internal networks
	BanIfError       bool   // Ban requests if IP lookup fails

	// Country-based rules (ISO 3166-1 alpha-2 format)
	AllowedCountries []string // Whitelist of countries to allow
	BlockedCountries []string // Blocklist of countries to block

	// IP-based rules
	AllowedIPBlocks    []string // Whitelist of CIDR blocks
	BlockedIPBlocks    []string // Blocklist of CIDR blocks
	AllowedIPBlocksDir string   // Path to directory containing allowed CIDR block files (.txt)
	BlockedIPBlocksDir string   // Path to directory containing blocked CIDR block files (.txt)

	// Response settings
	DisallowedStatusCode int    // HTTP status code for blocked requests
	BanHtmlFilePath      string // Custom HTML template for blocked requests
	CountryHeader        string // Header to write the country code to

	// Logging configuration
	LogLevel                    string // Log level: "debug", "info", "warn", "error"
	LogFormat                   string // Log format: "json" or "text"
	LogPath                     string // Log destination: "stdout", "stderr", or file path
	LogBannedRequests           bool   // Log blocked requests
	FileLogBufferSizeBytes      int    // Buffer size for file logging in bytes (default: 1024)
	FileLogBufferTimeoutSeconds int    // Buffer timeout for file logging in seconds (default: 2)

	// BypassHeaders is a map of header names to values that, when matched,
	// will skip the geoblocking check entirely
	BypassHeaders map[string]string

	// IP extraction settings
	IPHeaders []string // List of headers to check for client IP addresses (cannot be empty)

	// Auto-update settings
	DatabaseAutoUpdate      bool   `json:"databaseAutoUpdate,omitempty"`
	DatabaseAutoUpdateDir   string `json:"databaseAutoUpdateDir,omitempty"`
	DatabaseAutoUpdateToken string `json:"databaseAutoUpdateToken,omitempty"`
	DatabaseAutoUpdateCode  string `json:"databaseAutoUpdateCode,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		DisallowedStatusCode:        http.StatusForbidden,
		LogLevel:                    "info",                                   // Default to info logging
		LogFormat:                   "text",                                   // Default to text format
		LogPath:                     "",                                       // Default to traefik
		BanIfError:                  true,                                     // Default to banning on errors
		BypassHeaders:               make(map[string]string),                  // Initialize empty map
		IPHeaders:                   []string{"x-forwarded-for", "x-real-ip"}, // Default IP headers
		DatabaseAutoUpdateCode:      "DB1",                                    // Default database code
		LogBannedRequests:           true,                                     // Default to logging blocked requests
		CountryHeader:               "",                                       // Default to empty thus not setting the header
		FileLogBufferSizeBytes:      1024,                                     // Default buffer size 1024 bytes
		FileLogBufferTimeoutSeconds: 2,                                        // Default timeout 2 seconds
	}
}

// Update the Plugin struct to store the ban HTML content instead of template
type Plugin struct {
	next                 http.Handler
	name                 string
	databaseFile         string           // Just for testing purposes
	db                   *DatabaseWrapper // Changed from ip2location.DB to DatabaseWrapper
	enabled              bool
	allowedCountries     map[string]struct{} // Instead of []string to improve lookup performance
	blockedCountries     map[string]struct{} // Instead of []string to improve lookup performance
	defaultAllow         bool
	allowPrivate         bool
	banIfError           bool
	disallowedStatusCode int
	allowedIPBlocks      *IpLookupFileMonitor // Fast radix tree-based allowed IP block lookups
	blockedIPBlocks      *IpLookupFileMonitor // Fast radix tree-based blocked IP block lookups
	banHtmlContent       string               // Changed from banHtmlTemplate
	logger               *slog.Logger
	bypassHeaders        map[string]string
	ipHeaders            []string // List of headers to check for client IP addresses
	logBannedRequests    bool
	countryHeader        string
}

// New creates a new plugin instance.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	bootstrapLogger := createBootstrapLogger(name)

	if next == nil {
		return nil, fmt.Errorf("%s: no next handler provided", name)
	}

	if cfg == nil {
		return nil, fmt.Errorf("%s: no config provided", name)
	}

	// Create logger first so we can use it for debugging
	logger := createLogger(name, cfg.LogLevel, cfg.LogFormat, cfg.LogPath, cfg.FileLogBufferSizeBytes, cfg.FileLogBufferTimeoutSeconds, bootstrapLogger)
	logger.Debug("initializing plugin",
		"logLevel", cfg.LogLevel,
		"logFormat", cfg.LogFormat,
		"logPath", cfg.LogPath)

	if !cfg.Enabled {
		bootstrapLogger.Warn("plugin disabled")
		return &Plugin{
			next:    next,
			name:    name,
			db:      nil,
			enabled: false,
			logger:  logger,
		}, nil
	}

	if http.StatusText(cfg.DisallowedStatusCode) == "" {
		return nil, fmt.Errorf("%s: %d is not a valid http status code", name, cfg.DisallowedStatusCode)
	}

	// Validate that IPHeaders is not empty
	if len(cfg.IPHeaders) == 0 {
		return nil, fmt.Errorf("%s: IPHeaders cannot be empty - at least one header must be specified for IP extraction", name)
	}

	// Create database configuration
	dbConfig := &DatabaseConfig{
		DatabaseFilePath:        cfg.DatabaseFilePath,
		DatabaseAutoUpdate:      cfg.DatabaseAutoUpdate,
		DatabaseAutoUpdateDir:   cfg.DatabaseAutoUpdateDir,
		DatabaseAutoUpdateToken: cfg.DatabaseAutoUpdateToken,
		DatabaseAutoUpdateCode:  cfg.DatabaseAutoUpdateCode,
	}

	// Get database factory - uses singleton pattern per database path
	// Using the bootstrap logger here because the database factory is shared between all plugins
	factory, err := GetDatabaseFactory(dbConfig, bootstrapLogger)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get database factory: %w", name, err)
	}

	// Get the database wrapper
	db := factory.GetWrapper()
	databasePath := db.GetPath()

	// Create separate IP lookup file monitors with radix trees for fast lookups and file monitoring
	allowedIPHelper, err := NewIpLookupFileMonitor(cfg.AllowedIPBlocks, cfg.AllowedIPBlocksDir, logger)
	if err != nil {
		return nil, fmt.Errorf("%s: failed loading allowed IP blocks: %w", name, err)
	}

	blockedIPHelper, err := NewIpLookupFileMonitor(cfg.BlockedIPBlocks, cfg.BlockedIPBlocksDir, logger)
	if err != nil {
		return nil, fmt.Errorf("%s: failed loading blocked IP blocks: %w", name, err)
	}

	var banHtmlContent string

	if cfg.BanHtmlFilePath != "" {
		cfg.BanHtmlFilePath = searchFile(cfg.BanHtmlFilePath, "geoblockban.html", bootstrapLogger)
		content, err := os.ReadFile(cfg.BanHtmlFilePath)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to load ban HTML file %s: %w", name, cfg.BanHtmlFilePath, err)
		} else {
			banHtmlContent = string(content)
		}
	}

	// Convert slices to maps for O(1) lookup
	allowedCountries := make(map[string]struct{}, len(cfg.AllowedCountries))
	for _, c := range cfg.AllowedCountries {
		allowedCountries[c] = struct{}{}
	}

	blockedCountries := make(map[string]struct{}, len(cfg.BlockedCountries))
	for _, c := range cfg.BlockedCountries {
		blockedCountries[c] = struct{}{}
	}

	plugin := &Plugin{
		next:                 next,
		name:                 name,
		databaseFile:         databasePath,
		db:                   db,
		enabled:              cfg.Enabled,
		allowedCountries:     allowedCountries,
		blockedCountries:     blockedCountries,
		defaultAllow:         cfg.DefaultAllow,
		allowPrivate:         cfg.AllowPrivate,
		banIfError:           cfg.BanIfError,
		disallowedStatusCode: cfg.DisallowedStatusCode,
		allowedIPBlocks:      allowedIPHelper,
		blockedIPBlocks:      blockedIPHelper,
		banHtmlContent:       banHtmlContent,
		bypassHeaders:        cfg.BypassHeaders,
		ipHeaders:            cfg.IPHeaders,
		logger:               logger,
		logBannedRequests:    cfg.LogBannedRequests,
		countryHeader:        cfg.CountryHeader,
	}

	return plugin, nil
}

// ServeHTTP implements the http.Handler interface.
func (p Plugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !p.enabled {
		p.logger.Debug("plugin disabled, passing request through")
		p.next.ServeHTTP(rw, req)
		return
	}

	// Check for bypass headers
	// Optimize by avoiding multiple map lookups and method calls
	for header, expectedValue := range p.bypassHeaders {
		if actualValue := req.Header.Get(header); actualValue == expectedValue {
			p.logger.Debug("bypassing geoblock due to bypass header match",
				"header", header,
				"value", expectedValue,
				"remote_addr", req.RemoteAddr,
				"x_real_ip", req.Header.Get("x-real-ip"),
				"x_forwarded_for", req.Header.Get("x-forwarded-for"))
			p.next.ServeHTTP(rw, req)
			return
		}
	}

	remoteIPs := p.GetRemoteIPs(req)

	for _, ip := range remoteIPs {
		allowed, country, phase, err := p.CheckAllowed(ip)

		if p.countryHeader != "" && country != "" {
			req.Header.Set(p.countryHeader, country)
		}
		if err != nil {
			var ipChain string = ""
			if len(remoteIPs) > 1 {
				ipChain = strings.Join(remoteIPs, ", ")
			}
			p.logger.Error("request check failed",
				"ip", ip,
				"ip_chain", ipChain,
				"host", req.Host,
				"method", req.Method,
				"path", req.URL.Path,
				"phase", phase,
				"error", err)

			if p.banIfError {
				p.serveBanHtml(rw, ip, "Unknown")
				return
			}
			// keel looping
			continue
		}
		if !allowed {
			var ipChain string = ""
			if len(remoteIPs) > 1 {
				ipChain = strings.Join(remoteIPs, ", ")
			}
			if p.logBannedRequests {
				p.logger.Info("blocked request",
					"ip", ip,
					"ip_chain", ipChain,
					"country", country,
					"host", req.Host,
					"method", req.Method,
					"phase", phase,
					"path", req.URL.Path)
			}
			p.serveBanHtml(rw, ip, country)
			return
		}
	}

	p.next.ServeHTTP(rw, req)
}

// GetRemoteIPs collects the remote IPs from the configured IP headers.
func (p Plugin) GetRemoteIPs(req *http.Request) []string {
	uniqIPs := make(map[string]struct{})

	// Check each configured IP header
	for _, headerName := range p.ipHeaders {
		if headerValue := req.Header.Get(headerName); headerValue != "" {
			for _, ip := range strings.Split(headerValue, ",") {
				ip = cleanIPAddress(ip)
				if ip == "" {
					continue
				}
				uniqIPs[ip] = struct{}{}
			}
		}
	}

	var ips []string
	for ip := range uniqIPs {
		ips = append(ips, ip)
	}

	return ips
}

func cleanIPAddress(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	// Split IP from port if port exists (e.g., "192.168.1.1:8080")
	host, _, err := net.SplitHostPort(ip)
	if err == nil {
		return host
	}
	return ip // If no port, return the original IP
}

// CheckAllowed determines if an IP address should be allowed through based on configured rules.
// Returns:
// - allow: whether the request should be allowed
// - country: the detected country code for the IP
// - err: any errors encountered during the check
// - phase: the phase in the verification process where the decision was made
func (p Plugin) CheckAllowed(ip string) (allow bool, country string, phase string, err error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false, ip, "", fmt.Errorf("unable to parse IP address from [%s]", ip)
	}

	if ipAddr.IsPrivate() {
		if p.allowPrivate {
			return true, PrivateIpCountryAlias, "allow_private", nil
		} else {
			return false, PrivateIpCountryAlias, "allow_private", nil
		}
	}

	// Look up the country for this IP first, so we have it available for all code paths
	country, err = p.Lookup(ip)
	if err != nil {
		return false, ip, "", fmt.Errorf("lookup of %s failed: %w", ip, err)
	}

	blocked, blockedNetworkLength, err := p.isBlockedIPBlocks(ipAddr)
	if err != nil {
		return false, country, "", fmt.Errorf("failed to check if IP %q is blocked by IP block: %w", ip, err)
	}

	allowed, allowedNetworkLength, err := p.isAllowedIPBlocks(ipAddr)
	if err != nil {
		return false, country, "", fmt.Errorf("failed to check if IP %q is allowed by IP block: %w", ip, err)
	}

	// NB: whichever matched prefix is longer has higher priority: more specific to less specific only if both matched.
	if (allowedNetworkLength < blockedNetworkLength) && (allowedNetworkLength > 0) && (blockedNetworkLength > 0) {
		if blocked {
			return false, country, "blocked_ip_block", nil
		}
		if allowed {
			return true, country, "allowed_ip_block", nil
		}
	} else {
		if allowed {
			return true, country, "allowed_ip_block", nil
		}
		if blocked {
			return false, country, "blocked_ip_block", nil
		}
	}

	if _, allowed := p.allowedCountries[country]; allowed {
		return true, country, "allowed_country", nil
	}

	if _, blocked := p.blockedCountries[country]; blocked {
		return false, country, "blocked_country", nil
	}

	if p.defaultAllow {
		return true, country, "default_allow", nil
	}
	return false, country, "default_allow", nil
}

// Lookup queries the ip2location database for a given IP address.
func (p Plugin) Lookup(ip string) (string, error) {
	record, err := p.db.Get_country_short(ip)
	if err != nil {
		return "", err
	}

	// Avoid redundant assignment and string conversion
	if strings.HasPrefix(strings.ToLower(record.Country_short), "invalid") {
		return "", errors.New(record.Country_short)
	}

	return record.Country_short, nil
}

// isAllowedIPBlocks checks if an IP is allowed based on the allowed CIDR blocks using fast radix tree lookup
func (p Plugin) isAllowedIPBlocks(ipAddr net.IP) (bool, int, error) {
	return p.allowedIPBlocks.IsContained(ipAddr)
}

// isBlockedIPBlocks checks if an IP is blocked based on the blocked CIDR blocks using fast radix tree lookup
func (p Plugin) isBlockedIPBlocks(ipAddr net.IP) (bool, int, error) {
	return p.blockedIPBlocks.IsContained(ipAddr)
}

// Update the serveBanHtml function to use simple string replacement
func (p Plugin) serveBanHtml(rw http.ResponseWriter, ip, country string) {
	if p.banHtmlContent != "" {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.WriteHeader(p.disallowedStatusCode)

		// Simple string replacements
		content := p.banHtmlContent
		content = strings.ReplaceAll(content, "{{.Country}}", country)
		content = strings.ReplaceAll(content, "{{.IP}}", ip)

		if _, err := rw.Write([]byte(content)); err != nil {
			p.logger.Warn("failed to write ban HTML response", "error", err)
		}
		return
	}
	rw.WriteHeader(p.disallowedStatusCode)
}
