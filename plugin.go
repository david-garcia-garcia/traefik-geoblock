package traefik_geoblock

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/ip2location"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
)

//go:generate go run ./tools/dbdownload/main.go -o ./IP2LOCATION-LITE-DB1.IPV6.BIN

const (
	PrivateIpCountryAlias       = "PRIVATE"
	DatabaseProviderIP2Location = "ip2location"
)

// Log status constants for observability headers
const (
	LogStatusPass  = "pass"
	LogStatusBlock = "block"
)

// Phase constants for logging and testing
const (
	PhaseNone           = "none" // No specific rule matched (e.g., no IPs found)
	PhaseAllowPrivate   = "allow_private"
	PhaseBlockedIPBlock = "blocked_ip_block"
	PhaseAllowedIPBlock = "allowed_ip_block"
	PhaseAllowedCountry = "allowed_country"
	PhaseBlockedCountry = "blocked_country"
	PhaseDefaultAllow   = "default_allow"
	PhaseIgnoreVerb     = "ignore_verb"
	PhaseExcludedRegex  = "excluded_regex"
	PhaseBypassHeader   = "bypass_header"
)

// IP header strategy constants
const (
	IPHeaderStrategyCheckAll              = "CheckAll"
	IPHeaderStrategyCheckFirst            = "CheckFirst"
	IPHeaderStrategyCheckFirstNonePrivate = "CheckFirstNonePrivate"
)

// Config defines the plugin configuration.
type Config struct {
	// Core settings
	Enabled      bool // Enable/disable the plugin
	DefaultAllow bool // Default behavior when IP matches no rules
	AllowPrivate bool // Allow requests from private/internal networks
	BanIfError   bool // Ban requests if IP lookup fails

	// Database provider. Empty defaults to ip2location. Only ip2location is implemented.
	DatabaseProvider string `json:"databaseProvider,omitempty" mapstructure:"databaseProvider"`

	// IP2Location provider settings. Each vendor keeps its own prefixed keys.
	Ip2locationDatabaseFilePath        string `json:"ip2location_databaseFilePath,omitempty" mapstructure:"ip2location_databaseFilePath"`
	Ip2locationDatabaseAutoUpdate      bool   `json:"ip2location_databaseAutoUpdate,omitempty" mapstructure:"ip2location_databaseAutoUpdate"`
	Ip2locationDatabaseAutoUpdateDir   string `json:"ip2location_databaseAutoUpdateDir,omitempty" mapstructure:"ip2location_databaseAutoUpdateDir"`
	Ip2locationDatabaseAutoUpdateToken string `json:"ip2location_databaseAutoUpdateToken,omitempty" mapstructure:"ip2location_databaseAutoUpdateToken"`
	Ip2locationDatabaseAutoUpdateCode  string `json:"ip2location_databaseAutoUpdateCode,omitempty" mapstructure:"ip2location_databaseAutoUpdateCode"`

	// Deprecated unprefixed aliases. Copied onto the ip2location_ fields when those are unset.
	DatabaseFilePath        string `json:"databaseFilePath,omitempty" mapstructure:"databaseFilePath"`
	DatabaseAutoUpdate      bool   `json:"databaseAutoUpdate,omitempty" mapstructure:"databaseAutoUpdate"`
	DatabaseAutoUpdateDir   string `json:"databaseAutoUpdateDir,omitempty" mapstructure:"databaseAutoUpdateDir"`
	DatabaseAutoUpdateToken string `json:"databaseAutoUpdateToken,omitempty" mapstructure:"databaseAutoUpdateToken"`
	DatabaseAutoUpdateCode  string `json:"databaseAutoUpdateCode,omitempty" mapstructure:"databaseAutoUpdateCode"`

	// Country-based rules (ISO 3166-1 alpha-2 format)
	AllowedCountries []string // Whitelist of countries to allow
	BlockedCountries []string // Blocklist of countries to block

	// IP-based rules
	AllowedIPBlocks    []string // Whitelist of CIDR blocks
	BlockedIPBlocks    []string // Blocklist of CIDR blocks
	AllowedIPBlocksDir string   // Path to directory containing allowed CIDR block files (.txt)
	BlockedIPBlocksDir string   // Path to directory containing blocked CIDR block files (.txt)

	// Response settings
	DisallowedStatusCode  int    // HTTP status code for blocked requests
	BanHtmlFilePath       string // Custom HTML template for blocked requests
	CountryHeader         string // Header to write the country code to (on REQUEST)
	LogStatusDetailHeader string // Header to write detailed status to (on REQUEST): "pass:{reason}" or "block:{reason}"

	// Logging configuration
	LogLevel  string // Log level: "debug", "info", "warn", "error"
	LogFormat string // Log format: "json" or "text"

	// BypassHeaders is a map of header names to values that, when matched,
	// will skip the geoblocking check entirely
	BypassHeaders map[string]string

	// IP extraction settings
	IPHeaders        []string // List of headers to check for client IP addresses (cannot be empty)
	IPHeaderStrategy string   // Strategy for processing multiple IP addresses: "CheckAll", "CheckFirst", "CheckFirstNonePrivate"

	// HTTP verb filtering
	IgnoreVerbs []string // List of HTTP verbs to ignore for blocking (still enriched with GeoIP)

	// Path exclusion
	ExcludedPathsRegex string // Regular expression to match paths that should skip blocking (still enriched with GeoIP)
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		DisallowedStatusCode:              http.StatusForbidden,
		LogLevel:                          "info",                                   // Default to info logging
		LogFormat:                         "text",                                   // Default to text format
		BanIfError:                        true,                                     // Default to banning on errors
		BypassHeaders:                     make(map[string]string),                  // Initialize empty map
		IPHeaders:                         []string{"x-forwarded-for", "x-real-ip"}, // Default IP headers
		IPHeaderStrategy:                  IPHeaderStrategyCheckAll,                 // Default to checking all IPs
		DatabaseProvider: DatabaseProviderIP2Location, // Only implemented provider
		CountryHeader:    "",                          // Default to empty thus not setting the header
	}
}

// Update the Plugin struct to store the ban HTML content instead of template
type Plugin struct {
	next                  http.Handler
	name                  string
	db                    dbprovider.Provider
	enabled               bool
	allowedCountries      map[string]struct{} // Instead of []string to improve lookup performance
	blockedCountries      map[string]struct{} // Instead of []string to improve lookup performance
	defaultAllow          bool
	allowPrivate          bool
	banIfError            bool
	disallowedStatusCode  int
	allowedIPBlocks       *iplookup.IpLookupFileMonitor
	blockedIPBlocks       *iplookup.IpLookupFileMonitor
	banHtmlContent        string // Changed from banHtmlTemplate
	logger                *slog.Logger
	bypassHeaders         map[string]string
	ipHeaders             []string            // List of headers to check for client IP addresses
	ipHeaderStrategy      string              // Strategy for processing multiple IP addresses
	ignoreVerbs           map[string]struct{} // Set of HTTP verbs to ignore for blocking
	excludedPathsRegex    *regexp.Regexp      // Compiled regex for excluded paths
	countryHeader         string
	logStatusDetailHeader string
}

// New creates a new plugin instance.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("%s: no next handler provided", name)
	}

	if cfg == nil {
		return nil, fmt.Errorf("%s: no config provided", name)
	}

	bootstrapLogger := logging.NewBootstrap(name, cfg.LogLevel)
	logger := logging.New(name, cfg.LogLevel, cfg.LogFormat, bootstrapLogger)
	logger.Debug("initializing plugin",
		"logLevel", cfg.LogLevel,
		"logFormat", cfg.LogFormat)

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

	// Validate IPHeaderStrategy
	if cfg.IPHeaderStrategy != IPHeaderStrategyCheckAll &&
		cfg.IPHeaderStrategy != IPHeaderStrategyCheckFirst &&
		cfg.IPHeaderStrategy != IPHeaderStrategyCheckFirstNonePrivate {
		return nil, fmt.Errorf("%s: invalid IPHeaderStrategy '%s', must be one of: %s, %s, %s",
			name, cfg.IPHeaderStrategy,
			IPHeaderStrategyCheckAll, IPHeaderStrategyCheckFirst, IPHeaderStrategyCheckFirstNonePrivate)
	}

	applyDeprecatedIP2LocationSettings(cfg, bootstrapLogger)

	// Bootstrap logger: the provider/factory is shared between plugin instances.
	db, err := openDatabaseProvider(cfg, bootstrapLogger)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get database provider: %w", name, err)
	}

	allowedIPHelper, err := iplookup.NewIpLookupFileMonitor(cfg.AllowedIPBlocks, cfg.AllowedIPBlocksDir, logger)
	if err != nil {
		return nil, fmt.Errorf("%s: failed loading allowed IP blocks: %w", name, err)
	}

	blockedIPHelper, err := iplookup.NewIpLookupFileMonitor(cfg.BlockedIPBlocks, cfg.BlockedIPBlocksDir, logger)
	if err != nil {
		return nil, fmt.Errorf("%s: failed loading blocked IP blocks: %w", name, err)
	}

	var banHtmlContent string

	if cfg.BanHtmlFilePath != "" {
		var err error
		cfg.BanHtmlFilePath, err = fileutils.Default.Search(cfg.BanHtmlFilePath, "geoblockban.html", logger)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to find ban HTML file: %w", name, err)
		}
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

	// Convert ignore verbs to map for O(1) lookup, normalize to uppercase
	ignoreVerbs := make(map[string]struct{}, len(cfg.IgnoreVerbs))
	for _, verb := range cfg.IgnoreVerbs {
		ignoreVerbs[strings.ToUpper(verb)] = struct{}{}
	}

	// Compile excluded paths regex if provided
	var excludedPathsRegex *regexp.Regexp
	if cfg.ExcludedPathsRegex != "" {
		var err error
		excludedPathsRegex, err = regexp.Compile(cfg.ExcludedPathsRegex)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid excludedPathsRegex pattern %q: %w", name, cfg.ExcludedPathsRegex, err)
		}
		logger.Debug("compiled excludedPathsRegex", "pattern", cfg.ExcludedPathsRegex)
	}

	plugin := &Plugin{
		next:                  next,
		name:                  name,
		db:                    db,
		enabled:               cfg.Enabled,
		allowedCountries:      allowedCountries,
		blockedCountries:      blockedCountries,
		defaultAllow:          cfg.DefaultAllow,
		allowPrivate:          cfg.AllowPrivate,
		banIfError:            cfg.BanIfError,
		disallowedStatusCode:  cfg.DisallowedStatusCode,
		allowedIPBlocks:       allowedIPHelper,
		blockedIPBlocks:       blockedIPHelper,
		banHtmlContent:        banHtmlContent,
		bypassHeaders:         cfg.BypassHeaders,
		ipHeaders:             cfg.IPHeaders,
		ipHeaderStrategy:      cfg.IPHeaderStrategy,
		ignoreVerbs:           ignoreVerbs,
		excludedPathsRegex:    excludedPathsRegex,
		logger:                logger,
		countryHeader:         cfg.CountryHeader,
		logStatusDetailHeader: cfg.LogStatusDetailHeader,
	}

	return plugin, nil
}

// applyDeprecatedIP2LocationSettings copies unprefixed IP2Location keys onto the
// ip2location_ fields when those are unset, then defaults the database code to DB1.
// Prefixed values win. CreateConfig must not pre-fill the new code field or an
// old-only databaseAutoUpdateCode would be ignored after Traefik decode.
func applyDeprecatedIP2LocationSettings(cfg *Config, logger *slog.Logger) {
	used := make([]string, 0, 5)
	if cfg.DatabaseFilePath != "" {
		used = append(used, "databaseFilePath")
		if cfg.Ip2locationDatabaseFilePath == "" {
			cfg.Ip2locationDatabaseFilePath = cfg.DatabaseFilePath
		}
	}
	if cfg.DatabaseAutoUpdate {
		used = append(used, "databaseAutoUpdate")
		if !cfg.Ip2locationDatabaseAutoUpdate {
			cfg.Ip2locationDatabaseAutoUpdate = true
		}
	}
	if cfg.DatabaseAutoUpdateDir != "" {
		used = append(used, "databaseAutoUpdateDir")
		if cfg.Ip2locationDatabaseAutoUpdateDir == "" {
			cfg.Ip2locationDatabaseAutoUpdateDir = cfg.DatabaseAutoUpdateDir
		}
	}
	if cfg.DatabaseAutoUpdateToken != "" {
		used = append(used, "databaseAutoUpdateToken")
		if cfg.Ip2locationDatabaseAutoUpdateToken == "" {
			cfg.Ip2locationDatabaseAutoUpdateToken = cfg.DatabaseAutoUpdateToken
		}
	}
	if cfg.DatabaseAutoUpdateCode != "" {
		used = append(used, "databaseAutoUpdateCode")
		if cfg.Ip2locationDatabaseAutoUpdateCode == "" {
			cfg.Ip2locationDatabaseAutoUpdateCode = cfg.DatabaseAutoUpdateCode
		}
	}
	if len(used) > 0 {
		logger.Warn("deprecated IP2Location settings are set; use the ip2location_ prefixed keys",
			"deprecated", used)
	}
	if cfg.Ip2locationDatabaseAutoUpdateCode == "" {
		cfg.Ip2locationDatabaseAutoUpdateCode = "DB1"
	}
}

// openDatabaseProvider constructs the geo DatabaseProvider selected by Config.
// Empty DatabaseProvider defaults to ip2location. Unknown values fail.
func openDatabaseProvider(cfg *Config, logger *slog.Logger) (dbprovider.Provider, error) {
	name := strings.TrimSpace(cfg.DatabaseProvider)
	if name == "" {
		name = DatabaseProviderIP2Location
	}
	switch strings.ToLower(name) {
	case DatabaseProviderIP2Location:
		return ip2location.New(ip2location.DatabaseConfig{
			DatabaseFilePath:        cfg.Ip2locationDatabaseFilePath,
			DatabaseAutoUpdate:      cfg.Ip2locationDatabaseAutoUpdate,
			DatabaseAutoUpdateDir:   cfg.Ip2locationDatabaseAutoUpdateDir,
			DatabaseAutoUpdateToken: cfg.Ip2locationDatabaseAutoUpdateToken,
			DatabaseAutoUpdateCode:  cfg.Ip2locationDatabaseAutoUpdateCode,
		}, logger)
	default:
		return nil, fmt.Errorf("unsupported database provider %q", cfg.DatabaseProvider)
	}
}

// isExcludedByRegex checks if the request matches the excluded paths regex.
// The regex is matched against "{host}{path}" (e.g., "example.com/api/users").
// Go's regexp package uses RE2 which guarantees linear time complexity,
// making it inherently safe from ReDoS attacks.
func (p Plugin) isExcludedByRegex(host, path string) bool {
	if p.excludedPathsRegex == nil {
		return false
	}
	return p.excludedPathsRegex.MatchString(host + path)
}

// setLogHeaders sets the log status headers on the request for observability.
// status is "pass" or "block", reason is the detailed reason (e.g., "allowed_country").
func (p Plugin) setLogHeaders(req *http.Request, status, reason string) {
	if p.logStatusDetailHeader != "" {
		req.Header.Set(p.logStatusDetailHeader, status+":"+reason)
	}
}

// ServeHTTP implements the http.Handler interface.
func (p Plugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !p.enabled {
		p.logger.Debug("plugin disabled, passing request through")
		p.next.ServeHTTP(rw, req)
		return
	}

	// Get list of unique remote IPs
	remoteIPs := p.GetRemoteIPs(req)
	var ipChain string = strings.Join(remoteIPs, ", ")

	// passReason tracks why the request is allowed to pass.
	// If set early (by bypass conditions), blocking is skipped but enrichment still happens.
	// Otherwise, geo-blocking rules are evaluated and passReason is set from CheckAllowed.
	// Initialized to PhaseNone in case no IPs are found to process.
	passReason := PhaseNone

	// Check if this HTTP verb should be ignored for blocking (but still enriched)
	if passReason == PhaseNone {
		if _, ignored := p.ignoreVerbs[strings.ToUpper(req.Method)]; ignored {
			passReason = PhaseIgnoreVerb
			p.logger.Debug("HTTP verb ignored for blocking",
				"method", req.Method,
				"remote_addr", req.RemoteAddr,
				"ip_chain", ipChain)
		}
	}

	// Check if request matches excluded paths regex (skip blocking but still enrich)
	// Matches against "{host}{path}" (e.g., "example.com/api/users")
	if passReason == PhaseNone {
		if p.isExcludedByRegex(req.Host, req.URL.Path) {
			passReason = PhaseExcludedRegex
			p.logger.Debug("request excluded from blocking by regex",
				"host", req.Host,
				"path", req.URL.Path,
				"remote_addr", req.RemoteAddr,
				"ip_chain", ipChain)
		}
	}

	// Check for bypass headers
	if passReason == PhaseNone {
		for header, expectedValue := range p.bypassHeaders {
			if actualValue := req.Header.Get(header); actualValue == expectedValue {
				p.logger.Debug("bypassing geoblock due to bypass header match",
					"header", header,
					"value", expectedValue,
					"remote_addr", req.RemoteAddr,
					"ip_chain", ipChain)
				passReason = PhaseBypassHeader
				break
			}
		}
	}

	// Process IPs based on strategy
	var foundPublicIP bool = false
	var countryHeaderSet bool = false

	// Set country header to PRIVATE initially - will be overridden by real countries
	if p.countryHeader != "" {
		req.Header.Set(p.countryHeader, PrivateIpCountryAlias)
	}

	for i, ip := range remoteIPs {
		// Apply strategy logic
		if p.ipHeaderStrategy == IPHeaderStrategyCheckFirst && i > 0 {
			break // Only check first IP
		}

		// For CheckFirstNonePrivate, skip private IPs unless no public IP has been found
		if p.ipHeaderStrategy == IPHeaderStrategyCheckFirstNonePrivate {
			ipAddr := net.ParseIP(ip)
			isPrivate := ipAddr != nil && (ipAddr.IsPrivate() || ipAddr.IsLoopback())

			if isPrivate && !foundPublicIP && i < len(remoteIPs)-1 {
				// Skip this private IP, but continue looking for public IPs
				continue
			}
			if !isPrivate {
				foundPublicIP = true
			}
		}

		allowed, country, phase, err := p.CheckAllowed(ip)

		// Override country header only with the first real (non-private) country we encounter
		if p.countryHeader != "" && country != "" && country != PrivateIpCountryAlias && !countryHeaderSet {
			req.Header.Set(p.countryHeader, country)
			countryHeaderSet = true
		}

		if err != nil {
			p.logger.Error("request check failed",
				"ip", ip,
				"ip_chain", ipChain,
				"host", req.Host,
				"method", req.Method,
				"path", req.URL.Path,
				"phase", phase,
				"error", err,
				"remote_addr", req.RemoteAddr)

			if p.banIfError && passReason == PhaseNone {
				p.setLogHeaders(req, LogStatusBlock, "error")
				p.serveBanHtml(rw, ip, "Unknown", "error", req.Method)
				return
			}
			// For non-CheckAll strategies, continue to next IP on error
			if p.ipHeaderStrategy != IPHeaderStrategyCheckAll {
				continue
			}
			continue
		}

		if !allowed && passReason == PhaseNone {
			p.setLogHeaders(req, LogStatusBlock, phase)
			p.serveBanHtml(rw, ip, country, phase, req.Method)
			return
		}

		// Update passReason from geo-check only if no bypass is active and request was allowed
		if passReason == PhaseNone && allowed {
			passReason = phase
		}

		// For CheckFirstNonePrivate, stop after processing first non-private IP
		if p.ipHeaderStrategy == IPHeaderStrategyCheckFirstNonePrivate && country != PrivateIpCountryAlias {
			break
		}
	}

	// Set log headers for allowed request
	p.setLogHeaders(req, LogStatusPass, passReason)

	p.next.ServeHTTP(rw, req)
}

// GetRemoteIPs collects the remote IPs from the configured IP headers.
// Headers are processed in the order defined in ipHeaders.
// Within each header, IPs are processed left-to-right (leftmost IP first)
// because the leftmost IP is typically the original client IP in proxy chains.
//
// Special synthetic header "remoteAddress" maps to req.RemoteAddr for direct access to the connection's remote address.
func (p Plugin) GetRemoteIPs(req *http.Request) []string {
	var ips []string
	seenIPs := make(map[string]struct{}) // For deduplication

	// Check each configured IP header in order
	for _, headerName := range p.ipHeaders {
		var headerValue string

		// Handle synthetic "remoteAddress" header
		if headerName == "remoteAddress" {
			headerValue = req.RemoteAddr
		} else {
			headerValue = req.Header.Get(headerName)
		}

		if headerValue != "" {
			// Process IPs within this header left-to-right (leftmost is original client)
			for _, ip := range strings.Split(headerValue, ",") {
				ip = cleanIPAddress(ip)
				if ip == "" {
					continue
				}
				// Only add if we haven't seen this IP before
				if _, seen := seenIPs[ip]; !seen {
					seenIPs[ip] = struct{}{}
					ips = append(ips, ip)
				}
			}
		}
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

	if ipAddr.IsPrivate() || ipAddr.IsLoopback() {
		if p.allowPrivate {
			return true, PrivateIpCountryAlias, PhaseAllowPrivate, nil
		} else {
			return false, PrivateIpCountryAlias, PhaseAllowPrivate, nil
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
			return false, country, PhaseBlockedIPBlock, nil
		}
		if allowed {
			return true, country, PhaseAllowedIPBlock, nil
		}
	} else {
		if allowed {
			return true, country, PhaseAllowedIPBlock, nil
		}
		if blocked {
			return false, country, PhaseBlockedIPBlock, nil
		}
	}

	if _, allowed := p.allowedCountries[country]; allowed {
		return true, country, PhaseAllowedCountry, nil
	}

	if _, blocked := p.blockedCountries[country]; blocked {
		return false, country, PhaseBlockedCountry, nil
	}

	if p.defaultAllow {
		return true, country, PhaseDefaultAllow, nil
	}
	return false, country, PhaseDefaultAllow, nil
}

// Lookup queries the ip2location database for a given IP address.
func (p Plugin) Lookup(ip string) (string, error) {
	return p.db.LookupCountry(ip)
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
func (p Plugin) serveBanHtml(rw http.ResponseWriter, ip, country, phase string, requestMethod string) {
	if p.banHtmlContent != "" && requestMethod == http.MethodGet {
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
