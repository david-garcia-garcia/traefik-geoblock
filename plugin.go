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
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/ipinfo"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
)

//go:generate go run ./tools/dbdownload/main.go -o ./IP2LOCATION-LITE-DB1.IPV6.BIN

const (
	PrivateIpCountryAlias       = "PRIVATE"
	EnrichNullAlias             = "null"
	DatabaseProviderIP2Location = "ip2location"
	DatabaseProviderIPinfo      = "ipinfo"
)

// Log status constants for observability headers
const (
	LogStatusPass  = "pass"
	LogStatusBlock = "block"
)

// Phase constants for logging and testing
const (
	PhaseNone             = "none" // No specific rule matched (e.g., no IPs found)
	PhaseAllowPrivate     = "allow_private"
	PhaseBlockedIPBlock   = "blocked_ip_block"
	PhaseAllowedIPBlock   = "allowed_ip_block"
	PhaseAllowedCountry   = "allowed_country"
	PhaseBlockedCountry   = "blocked_country"
	PhaseDefaultAllow     = "default_allow"
	PhaseIgnoreVerb       = "ignore_verb"
	PhaseExcludedRegex    = "excluded_regex"
	PhaseNotIncludedRegex = "not_included_regex"
	PhaseBypassHeader     = "bypass_header"
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

	// Database provider. Empty defaults to ip2location. Implemented: ip2location, ipinfo.
	DatabaseProvider string `json:"databaseProvider,omitempty" mapstructure:"databaseProvider"`

	// IP2Location provider settings. Each vendor keeps its own prefixed keys.
	Ip2locationDatabaseFilePath          string `json:"ip2location_databaseFilePath,omitempty" mapstructure:"ip2location_databaseFilePath"`
	Ip2locationDatabaseAutoUpdate        bool   `json:"ip2location_databaseAutoUpdate,omitempty" mapstructure:"ip2location_databaseAutoUpdate"`
	Ip2locationDatabaseAutoUpdateDir     string `json:"ip2location_databaseAutoUpdateDir,omitempty" mapstructure:"ip2location_databaseAutoUpdateDir"`
	Ip2locationDatabaseAutoUpdateToken   string `json:"ip2location_databaseAutoUpdateToken,omitempty" mapstructure:"ip2location_databaseAutoUpdateToken"`
	Ip2locationDatabaseAutoUpdateCode    string `json:"ip2location_databaseAutoUpdateCode,omitempty" mapstructure:"ip2location_databaseAutoUpdateCode"`
	Ip2locationAsnDatabaseFilePath       string `json:"ip2location_asnDatabaseFilePath,omitempty" mapstructure:"ip2location_asnDatabaseFilePath"`
	Ip2locationAsnDatabaseAutoUpdate     bool   `json:"ip2location_asnDatabaseAutoUpdate,omitempty" mapstructure:"ip2location_asnDatabaseAutoUpdate"`
	Ip2locationAsnDatabaseAutoUpdateCode string `json:"ip2location_asnDatabaseAutoUpdateCode,omitempty" mapstructure:"ip2location_asnDatabaseAutoUpdateCode"`

	// IPinfo Lite (MMDB). Auto-update requires a free-account token.
	IpinfoDatabaseFilePath        string `json:"ipinfo_databaseFilePath,omitempty" mapstructure:"ipinfo_databaseFilePath"`
	IpinfoDatabaseAutoUpdate      bool   `json:"ipinfo_databaseAutoUpdate,omitempty" mapstructure:"ipinfo_databaseAutoUpdate"`
	IpinfoDatabaseAutoUpdateDir   string `json:"ipinfo_databaseAutoUpdateDir,omitempty" mapstructure:"ipinfo_databaseAutoUpdateDir"`
	IpinfoDatabaseAutoUpdateToken string `json:"ipinfo_databaseAutoUpdateToken,omitempty" mapstructure:"ipinfo_databaseAutoUpdateToken"`

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
	CountryHeader         string // Deprecated: folded into RequestHeaderEnrich as key country
	LogStatusDetailHeader string // Header to write detailed status to (on REQUEST): "pass:{reason}" or "block:{reason}"

	// RequestHeaderEnrich maps request header names to metadata keys
	// (country, country_name, continent, continent_code, region, city, isp, domain, asn).
	// Empty or unavailable fields are written as EnrichNullAlias ("null").
	RequestHeaderEnrich map[string]string `json:"requestHeaderEnrich,omitempty" mapstructure:"requestHeaderEnrich"`

	// Logging configuration
	LogLevel  string // Log level: "trace", "debug", "info", "warn", "error"
	LogFormat string // Log format: "json" or "text"

	// BypassHeaders is a map of header names to values that, when matched,
	// will skip the geoblocking check entirely
	BypassHeaders map[string]string

	// IP extraction settings
	IPHeaders        []string // List of headers to check for client IP addresses (cannot be empty)
	IPHeaderStrategy string   // Strategy for processing multiple IP addresses: "CheckAll", "CheckFirst", "CheckFirstNonePrivate"

	// HTTP verb filtering
	IgnoreVerbs []string // List of HTTP verbs to ignore for blocking (still enriched with GeoIP)

	// Path inclusion / exclusion. Both match "{host}{path}" (e.g. example.com/api/users).
	// IncludedPathsRegex: when set, only matching requests are candidates for blocking.
	// ExcludedPathsRegex: matching requests skip blocking. Runs after include.
	IncludedPathsRegex string
	ExcludedPathsRegex string
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		DisallowedStatusCode: http.StatusForbidden,
		LogLevel:             "info",                  // Default to info logging
		LogFormat:            "text",                  // Default to text format
		BanIfError:           true,                    // Default to banning on errors
		BypassHeaders:        make(map[string]string), // Initialize empty map
		RequestHeaderEnrich:  make(map[string]string),
		IPHeaders:            []string{"x-forwarded-for", "x-real-ip"}, // Default IP headers
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,                 // Default to checking all IPs
		DatabaseProvider:     DatabaseProviderIP2Location,              // Default provider
		CountryHeader:        "",                                       // Default to empty thus not setting the header
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
	includedPathsRegex    *regexp.Regexp      // When set, only matching {host}{path} may be blocked
	excludedPathsRegex    *regexp.Regexp      // Matching {host}{path} skip blocking (after include)
	logStatusDetailHeader string
	requestHeaderEnrich   map[string]string // header name -> metadata key
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

	requestHeaderEnrich, err := normalizeRequestHeaderEnrich(cfg.RequestHeaderEnrich)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	requestHeaderEnrich = foldCountryHeader(cfg.CountryHeader, requestHeaderEnrich, logger)

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

	includedPathsRegex, err := compilePathRegex(name, "includedPathsRegex", cfg.IncludedPathsRegex)
	if err != nil {
		return nil, err
	}
	if includedPathsRegex != nil {
		logger.Debug("compiled includedPathsRegex", "pattern", cfg.IncludedPathsRegex)
	}
	excludedPathsRegex, err := compilePathRegex(name, "excludedPathsRegex", cfg.ExcludedPathsRegex)
	if err != nil {
		return nil, err
	}
	if excludedPathsRegex != nil {
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
		ipHeaders:             canonicalIPHeaders(cfg.IPHeaders),
		ipHeaderStrategy:      cfg.IPHeaderStrategy,
		ignoreVerbs:           ignoreVerbs,
		includedPathsRegex:    includedPathsRegex,
		excludedPathsRegex:    excludedPathsRegex,
		logger:                logger,
		logStatusDetailHeader: cfg.LogStatusDetailHeader,
		requestHeaderEnrich:   requestHeaderEnrich,
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
	if cfg.Ip2locationAsnDatabaseAutoUpdateCode == "" {
		cfg.Ip2locationAsnDatabaseAutoUpdateCode = ip2location.DefaultASNDatabaseCode
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
			DatabaseFilePath:          cfg.Ip2locationDatabaseFilePath,
			DatabaseAutoUpdate:        cfg.Ip2locationDatabaseAutoUpdate,
			DatabaseAutoUpdateDir:     cfg.Ip2locationDatabaseAutoUpdateDir,
			DatabaseAutoUpdateToken:   cfg.Ip2locationDatabaseAutoUpdateToken,
			DatabaseAutoUpdateCode:    cfg.Ip2locationDatabaseAutoUpdateCode,
			AsnDatabaseFilePath:       cfg.Ip2locationAsnDatabaseFilePath,
			AsnDatabaseAutoUpdate:     cfg.Ip2locationAsnDatabaseAutoUpdate,
			AsnDatabaseAutoUpdateCode: cfg.Ip2locationAsnDatabaseAutoUpdateCode,
		}, logger)
	case DatabaseProviderIPinfo:
		return ipinfo.New(ipinfo.DatabaseConfig{
			DatabaseFilePath:        cfg.IpinfoDatabaseFilePath,
			DatabaseAutoUpdate:      cfg.IpinfoDatabaseAutoUpdate,
			DatabaseAutoUpdateDir:   cfg.IpinfoDatabaseAutoUpdateDir,
			DatabaseAutoUpdateToken: cfg.IpinfoDatabaseAutoUpdateToken,
		}, logger)
	default:
		return nil, fmt.Errorf("unsupported database provider %q", cfg.DatabaseProvider)
	}
}

func compilePathRegex(pluginName, field, pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid %s pattern %q: %w", pluginName, field, pattern, err)
	}
	return re, nil
}

// isExcludedByRegex reports whether {host}{path} matches excludedPathsRegex.
func (p Plugin) isExcludedByRegex(host, path string) bool {
	return pathRegexMatches(p.excludedPathsRegex, host, path)
}

// isIncludedByRegex is true when includedPathsRegex is unset, or when {host}{path} matches it.
func (p Plugin) isIncludedByRegex(host, path string) bool {
	if p.includedPathsRegex == nil {
		return true
	}
	return pathRegexMatches(p.includedPathsRegex, host, path)
}

func pathRegexMatches(re *regexp.Regexp, host, path string) bool {
	if re == nil {
		return false
	}
	return re.MatchString(host + path)
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
		logging.Trace(p.logger, "plugin disabled, passing request through")
		p.next.ServeHTTP(rw, req)
		return
	}

	// Get list of unique remote IPs
	remoteIPs := p.GetRemoteIPs(req)
	ipChain := ""
	if p.logger.Enabled(context.Background(), logging.LevelTrace) {
		ipChain = strings.Join(remoteIPs, ", ")
	}

	// passReason tracks why the request is allowed to pass.
	// If set early (by bypass conditions), blocking is skipped but enrichment still happens.
	// Otherwise, geo-blocking rules are evaluated and passReason is set from CheckAllowed.
	// Initialized to PhaseNone in case no IPs are found to process.
	passReason := PhaseNone

	// Check if this HTTP verb should be ignored for blocking (but still enriched)
	if passReason == PhaseNone {
		if _, ignored := p.ignoreVerbs[strings.ToUpper(req.Method)]; ignored {
			passReason = PhaseIgnoreVerb
			logging.Trace(p.logger, "HTTP verb ignored for blocking",
				"method", req.Method,
				"remote_addr", req.RemoteAddr,
				"ip_chain", ipChain)
		}
	}

	// Include first: when set, only matching {host}{path} may be blocked.
	if passReason == PhaseNone && !p.isIncludedByRegex(req.Host, req.URL.Path) {
		passReason = PhaseNotIncludedRegex
		logging.Trace(p.logger, "request not included for blocking by regex",
			"host", req.Host,
			"path", req.URL.Path,
			"remote_addr", req.RemoteAddr,
			"ip_chain", ipChain)
	}

	// Exclude after include: matching requests still skip blocking and stay enriched.
	if passReason == PhaseNone && p.isExcludedByRegex(req.Host, req.URL.Path) {
		passReason = PhaseExcludedRegex
		logging.Trace(p.logger, "request excluded from blocking by regex",
			"host", req.Host,
			"path", req.URL.Path,
			"remote_addr", req.RemoteAddr,
			"ip_chain", ipChain)
	}

	// Check for bypass headers
	if passReason == PhaseNone {
		for header, expectedValue := range p.bypassHeaders {
			if actualValue := req.Header.Get(header); actualValue == expectedValue {
				logging.Trace(p.logger, "bypassing geoblock due to bypass header match",
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
	var geoHeaderSet bool = false

	p.setPrivateGeoHeaders(req)

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

		allowed, rec, phase, err := p.CheckAllowed(ip)
		country := rec.Country

		if !geoHeaderSet {
			p.applyGeoHeaders(req, rec, &geoHeaderSet)
		}

		if err != nil {
			if ipChain == "" {
				ipChain = strings.Join(remoteIPs, ", ")
			}
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
	var seenIPs map[string]struct{}

	for _, headerName := range p.ipHeaders {
		var headerValue string
		if headerName == "remoteAddress" {
			headerValue = req.RemoteAddr
		} else {
			headerValue = req.Header.Get(headerName)
		}
		if headerValue == "" {
			continue
		}

		for len(headerValue) > 0 {
			var part string
			if i := strings.IndexByte(headerValue, ','); i >= 0 {
				part = headerValue[:i]
				headerValue = headerValue[i+1:]
			} else {
				part = headerValue
				headerValue = ""
			}
			ip := cleanIPAddress(part)
			if ip == "" {
				continue
			}
			if len(ips) == 0 {
				ips = append(ips, ip)
				continue
			}
			if seenIPs == nil {
				seenIPs = make(map[string]struct{}, 2)
				seenIPs[ips[0]] = struct{}{}
			}
			if _, seen := seenIPs[ip]; seen {
				continue
			}
			seenIPs[ip] = struct{}{}
			ips = append(ips, ip)
		}
	}

	return ips
}

func canonicalIPHeaders(headers []string) []string {
	out := make([]string, len(headers))
	for i, headerName := range headers {
		if headerName == "remoteAddress" {
			out[i] = headerName
			continue
		}
		out[i] = http.CanonicalHeaderKey(headerName)
	}
	return out
}

func cleanIPAddress(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	// IPv4 without a port has no ':'. SplitHostPort allocates an error on that path.
	if strings.IndexByte(ip, ':') == -1 {
		return ip
	}
	host, _, err := net.SplitHostPort(ip)
	if err == nil {
		return host
	}
	return ip
}

// CheckAllowed determines if an IP address should be allowed through based on configured rules.
// Returns:
// - allow: whether the request should be allowed
// - country: the detected country code for the IP
// - err: any errors encountered during the check
// - phase: the phase in the verification process where the decision was made
func (p Plugin) CheckAllowed(ip string) (allow bool, rec dbprovider.Record, phase string, err error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false, dbprovider.Record{Country: ip}, "", fmt.Errorf("unable to parse IP address from [%s]", ip)
	}

	if ipAddr.IsPrivate() || ipAddr.IsLoopback() {
		private := dbprovider.Record{Country: PrivateIpCountryAlias}
		if p.allowPrivate {
			return true, private, PhaseAllowPrivate, nil
		}
		return false, private, PhaseAllowPrivate, nil
	}

	rec, err = p.Lookup(ip)
	if err != nil {
		return false, dbprovider.Record{Country: ip}, "", fmt.Errorf("lookup of %s failed: %w", ip, err)
	}
	country := rec.Country

	blocked, blockedNetworkLength, err := p.isBlockedIPBlocks(ipAddr)
	if err != nil {
		return false, rec, "", fmt.Errorf("failed to check if IP %q is blocked by IP block: %w", ip, err)
	}

	allowed, allowedNetworkLength, err := p.isAllowedIPBlocks(ipAddr)
	if err != nil {
		return false, rec, "", fmt.Errorf("failed to check if IP %q is allowed by IP block: %w", ip, err)
	}

	// NB: whichever matched prefix is longer has higher priority: more specific to less specific only if both matched.
	if (allowedNetworkLength < blockedNetworkLength) && (allowedNetworkLength > 0) && (blockedNetworkLength > 0) {
		if blocked {
			return false, rec, PhaseBlockedIPBlock, nil
		}
		if allowed {
			return true, rec, PhaseAllowedIPBlock, nil
		}
	} else {
		if allowed {
			return true, rec, PhaseAllowedIPBlock, nil
		}
		if blocked {
			return false, rec, PhaseBlockedIPBlock, nil
		}
	}

	if _, allowed := p.allowedCountries[country]; allowed {
		return true, rec, PhaseAllowedCountry, nil
	}

	if _, blocked := p.blockedCountries[country]; blocked {
		return false, rec, PhaseBlockedCountry, nil
	}

	if p.defaultAllow {
		return true, rec, PhaseDefaultAllow, nil
	}
	return false, rec, PhaseDefaultAllow, nil
}

// Lookup returns geo metadata for ip.
func (p Plugin) Lookup(ip string) (dbprovider.Record, error) {
	return p.db.Lookup(ip)
}

func normalizeRequestHeaderEnrich(in map[string]string) (map[string]string, error) {
	if len(in) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(in))
	for header, key := range in {
		k := strings.ToLower(strings.TrimSpace(key))
		if !dbprovider.KnownMetaKey(k) {
			return nil, fmt.Errorf("unknown requestHeaderEnrich metadata key %q (supported: %s)",
				key, strings.Join(dbprovider.MetaKeys(), ", "))
		}
		out[http.CanonicalHeaderKey(header)] = k
	}
	return out, nil
}

// foldCountryHeader copies deprecated countryHeader into requestHeaderEnrich as
// key country. A set enrich mapping for the same header name wins.
func foldCountryHeader(countryHeader string, enrich map[string]string, logger *slog.Logger) map[string]string {
	h := strings.TrimSpace(countryHeader)
	if h == "" {
		return enrich
	}
	logger.Warn("countryHeader is deprecated; use requestHeaderEnrich with metadata key country",
		"header", h)
	canon := http.CanonicalHeaderKey(h)
	if enrich == nil {
		enrich = map[string]string{}
	}
	if _, exists := enrich[canon]; !exists {
		enrich[canon] = dbprovider.MetaCountry
	}
	return enrich
}

func (p Plugin) setPrivateGeoHeaders(req *http.Request) {
	for header, key := range p.requestHeaderEnrich {
		if key == dbprovider.MetaCountry {
			req.Header.Set(header, PrivateIpCountryAlias)
			continue
		}
		req.Header.Set(header, EnrichNullAlias)
	}
}

func (p Plugin) applyGeoHeaders(req *http.Request, rec dbprovider.Record, set *bool) {
	if rec.Country == "" || rec.Country == PrivateIpCountryAlias {
		return
	}
	for header, key := range p.requestHeaderEnrich {
		value := rec.Field(key)
		if value == "" {
			value = EnrichNullAlias
		}
		req.Header.Set(header, value)
	}
	*set = true
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
