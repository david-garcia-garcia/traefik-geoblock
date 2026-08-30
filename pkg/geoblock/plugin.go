package geoblock

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/ip2location"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/ipinfo"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/maxmind"
)

const (
	PrivateIpCountryAlias = "PRIVATE"
	EnrichNullAlias       = "null"
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

// Plugin is the shared policy for one middleware name and config, including the database provider.
type Plugin struct {
	name                  string
	db                    dbprovider.Provider
	lifeCancel            context.CancelFunc
	closeOnce             *sync.Once
	mode                  string
	countryHeader         string
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

// pluginLogger is the slog logger for this middleware name and config.
func pluginLogger(name string, cfg *Config) *slog.Logger {
	bootstrap := logging.NewBootstrap(name, cfg.LogLevel)
	return logging.New(name, cfg.LogLevel, cfg.LogFormat, bootstrap)
}

// NewCore builds the Plugin and opens the database provider on this incarnation’s lifetime. cfg must already be Prepare'd.
func NewCore(name string, cfg *Config) (*Plugin, error) {
	logger := pluginLogger(name, cfg)
	logger.Debug("initializing plugin",
		"logLevel", cfg.LogLevel,
		"logFormat", cfg.LogFormat)
	mode := NormalizeMode(cfg.Mode)
	if mode == ModeDisabled {
		logger.Warn("plugin disabled")
		return &Plugin{name: name, mode: ModeDisabled, logger: logger, closeOnce: &sync.Once{}}, nil
	}

	requestHeaderEnrich, err := normalizeRequestHeaderEnrich(cfg.RequestHeaderEnrich)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if ModeLooksUp(mode) {
		requestHeaderEnrich = foldCountryHeader(cfg.CountryHeader, requestHeaderEnrich)
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
		content, err := os.ReadFile(cfg.BanHtmlFilePath)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to load ban HTML file %s: %w", name, cfg.BanHtmlFilePath, err)
		}
		banHtmlContent = string(content)
	}

	allowedCountries := make(map[string]struct{}, len(cfg.AllowedCountries))
	for _, c := range cfg.AllowedCountries {
		allowedCountries[c] = struct{}{}
	}

	blockedCountries := make(map[string]struct{}, len(cfg.BlockedCountries))
	for _, c := range cfg.BlockedCountries {
		blockedCountries[c] = struct{}{}
	}

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

	life, lifeCancel := context.WithCancel(context.Background())
	pluginInstance := &Plugin{
		name:                  name,
		lifeCancel:            lifeCancel,
		closeOnce:             &sync.Once{},
		mode:                  mode,
		countryHeader:         http.CanonicalHeaderKey(strings.TrimSpace(cfg.CountryHeader)),
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
	if ModeLooksUp(mode) {
		if err := pluginInstance.bindDatabase(life, cfg); err != nil {
			lifeCancel()
			return nil, err
		}
	}
	return pluginInstance, nil
}

// Close ends this incarnation’s lifetime so format wrappers drop this holder.
func (p *Plugin) Close() {
	if p == nil {
		return
	}
	if p.closeOnce == nil {
		return
	}
	p.closeOnce.Do(func() {
		if p.lifeCancel != nil {
			p.lifeCancel()
		}
	})
}

// bindDatabase opens the provider bound to this incarnation’s lifetime.
func (p *Plugin) bindDatabase(life context.Context, cfg *Config) error {
	if !ModeLooksUp(p.mode) {
		return nil
	}
	db, err := openDatabaseProvider(life, cfg, logging.NewBootstrap(p.name, cfg.LogLevel))
	if err != nil {
		return fmt.Errorf("%s: failed to get database provider: %w", p.name, err)
	}
	p.db = db
	return nil
}

// openDatabaseProvider constructs the geo DatabaseProvider selected by Config.
// Empty DatabaseProvider defaults to ip2location. Unknown values fail.
func openDatabaseProvider(ctx context.Context, cfg *Config, logger *slog.Logger) (dbprovider.Provider, error) {
	name := providerName(cfg)
	switch name {
	case DatabaseProviderIP2Location:
		return ip2location.New(ctx, ip2location.DatabaseConfig{
			DatabaseAutoUpdateDir: cfg.DatabaseAutoUpdateDir,
			Source:                catalogSource(cfg, cfg.Ip2locationSourceGeo, dbsource.TypeBIN),
			AsnSource:             catalogSource(cfg, cfg.Ip2locationSourceAsn, dbsource.TypeBIN),
		}, logger)
	case DatabaseProviderIPinfo:
		return ipinfo.New(ctx, ipinfo.DatabaseConfig{
			DatabaseAutoUpdateDir: cfg.DatabaseAutoUpdateDir,
			Source:                catalogSource(cfg, cfg.IpinfoSource, dbsource.TypeMMDB),
		}, logger)
	case DatabaseProviderMaxMind:
		return maxmind.New(ctx, maxmind.DatabaseConfig{
			DatabaseAutoUpdateDir: cfg.DatabaseAutoUpdateDir,
			Source:                catalogSource(cfg, cfg.MaxmindSource, dbsource.TypeMMDB),
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

// ServeHTTP applies this Plugin’s policy, then calls next.
func (p Plugin) ServeHTTP(rw http.ResponseWriter, req *http.Request, next http.Handler) {
	if p.mode == ModeDisabled {
		logging.Trace(p.logger, "plugin disabled, passing request through")
		next.ServeHTTP(rw, req)
		return
	}

	remoteIPs := p.GetRemoteIPs(req)
	ipChain := ""
	if p.logger.Enabled(context.Background(), logging.LevelTrace) {
		ipChain = strings.Join(remoteIPs, ", ")
	}

	// passReason skips the block stage when set by verb / path / bypass. Lookup still runs.
	passReason := PhaseNone
	if _, ignored := p.ignoreVerbs[strings.ToUpper(req.Method)]; ignored {
		passReason = PhaseIgnoreVerb
		logging.Trace(p.logger, "HTTP verb ignored for blocking",
			"method", req.Method, "remote_addr", req.RemoteAddr, "ip_chain", ipChain)
	}
	if passReason == PhaseNone && !p.isIncludedByRegex(req.Host, req.URL.Path) {
		passReason = PhaseNotIncludedRegex
		logging.Trace(p.logger, "request not included for blocking by regex",
			"host", req.Host, "path", req.URL.Path, "remote_addr", req.RemoteAddr, "ip_chain", ipChain)
	}
	if passReason == PhaseNone && p.isExcludedByRegex(req.Host, req.URL.Path) {
		passReason = PhaseExcludedRegex
		logging.Trace(p.logger, "request excluded from blocking by regex",
			"host", req.Host, "path", req.URL.Path, "remote_addr", req.RemoteAddr, "ip_chain", ipChain)
	}
	if passReason == PhaseNone {
		for header, expectedValue := range p.bypassHeaders {
			if actualValue := req.Header.Get(header); actualValue == expectedValue {
				logging.Trace(p.logger, "bypassing geoblock due to bypass header match",
					"header", header, "value", logging.Redact(expectedValue),
					"remote_addr", req.RemoteAddr, "ip_chain", ipChain)
				passReason = PhaseBypassHeader
				break
			}
		}
	}

	if ModeLooksUp(p.mode) {
		if p.lookupAndWriteHeaders(rw, req, remoteIPs, ipChain, passReason) {
			return
		}
	}

	if !ModeBlocks(p.mode) {
		next.ServeHTTP(rw, req)
		return
	}

	if passReason != PhaseNone {
		p.setLogHeaders(req, LogStatusPass, passReason)
		next.ServeHTTP(rw, req)
		return
	}

	if p.blockFromHeader(rw, req, remoteIPs, ipChain) {
		return
	}

	next.ServeHTTP(rw, req)
}

// lookupAndWriteHeaders runs the lookup stage. true means the request was already banned.
func (p Plugin) lookupAndWriteHeaders(rw http.ResponseWriter, req *http.Request, remoteIPs []string, ipChain, passReason string) bool {
	var foundPublicIP bool
	var geoHeaderSet bool
	p.setPrivateGeoHeaders(req)
	for i, ip := range remoteIPs {
		if p.skipIP(i, ip, remoteIPs, &foundPublicIP) {
			continue
		}
		rec, err := p.recordForLookup(ip)
		if !geoHeaderSet {
			p.applyGeoHeaders(req, rec, &geoHeaderSet)
		}
		if err != nil {
			p.logLookupError(req, ip, ipChain, remoteIPs, err)
			if ModeBlocks(p.mode) && p.banIfError && passReason == PhaseNone {
				p.setLogHeaders(req, LogStatusBlock, "error")
				p.serveBanHtml(rw, ip, "Unknown", req.Method)
				return true
			}
			continue
		}
		if p.ipHeaderStrategy == IPHeaderStrategyCheckFirstNonePrivate && rec.Country != PrivateIpCountryAlias {
			break
		}
	}
	return false
}

// blockFromHeader runs CIDR / private / country using countryHeader. true means banned.
func (p Plugin) blockFromHeader(rw http.ResponseWriter, req *http.Request, remoteIPs []string, ipChain string) bool {
	country := req.Header.Get(p.countryHeader)
	if country == "" || country == EnrichNullAlias {
		if p.banIfError {
			p.setLogHeaders(req, LogStatusBlock, "error")
			p.serveBanHtml(rw, firstIP(remoteIPs), "Unknown", req.Method)
			return true
		}
	}
	var foundPublicIP bool
	passReason := PhaseNone
	for i, ip := range remoteIPs {
		if p.skipIP(i, ip, remoteIPs, &foundPublicIP) {
			continue
		}
		allowed, rec, phase, err := p.decide(ip, country)
		if err != nil {
			p.logLookupError(req, ip, ipChain, remoteIPs, err)
			if p.banIfError && passReason == PhaseNone {
				p.setLogHeaders(req, LogStatusBlock, "error")
				p.serveBanHtml(rw, ip, "Unknown", req.Method)
				return true
			}
			continue
		}
		if !allowed && passReason == PhaseNone {
			p.setLogHeaders(req, LogStatusBlock, phase)
			p.serveBanHtml(rw, ip, rec.Country, req.Method)
			return true
		}
		if passReason == PhaseNone && allowed {
			passReason = phase
		}
		if p.ipHeaderStrategy == IPHeaderStrategyCheckFirstNonePrivate && rec.Country != PrivateIpCountryAlias {
			break
		}
	}
	p.setLogHeaders(req, LogStatusPass, passReason)
	return false
}

// skipIP reports whether this hop is skipped by ipHeaderStrategy.
func (p Plugin) skipIP(i int, ip string, remoteIPs []string, foundPublicIP *bool) bool {
	if p.ipHeaderStrategy == IPHeaderStrategyCheckFirst && i > 0 {
		return true
	}
	if p.ipHeaderStrategy != IPHeaderStrategyCheckFirstNonePrivate {
		return false
	}
	ipAddr := net.ParseIP(ip)
	isPrivate := ipAddr != nil && (ipAddr.IsPrivate() || ipAddr.IsLoopback())
	if isPrivate && !*foundPublicIP && i < len(remoteIPs)-1 {
		return true
	}
	if !isPrivate {
		*foundPublicIP = true
	}
	return false
}

// firstIP is the first extracted IP, or empty.
func firstIP(ips []string) string {
	if len(ips) == 0 {
		return ""
	}
	return ips[0]
}

// logLookupError writes the request-check failed log line.
func (p Plugin) logLookupError(req *http.Request, ip, ipChain string, remoteIPs []string, err error) {
	if ipChain == "" {
		ipChain = strings.Join(remoteIPs, ", ")
	}
	p.logger.Error("request check failed",
		"ip", ip, "ip_chain", ipChain, "host", req.Host,
		"method", req.Method, "path", req.URL.Path, "error", err,
		"remote_addr", req.RemoteAddr)
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

// CheckAllowed is private + CIDR + country from a lookup when a provider is bound.
// Request country rules use decide with countryHeader instead.
func (p Plugin) CheckAllowed(ip string) (allow bool, rec dbprovider.Record, phase string, err error) {
	rec, err = p.recordForLookup(ip)
	if err != nil {
		return false, rec, "", err
	}
	return p.decide(ip, rec.Country)
}

// recordForLookup is PRIVATE for private/loopback IPs, else DatabaseProvider Lookup.
func (p Plugin) recordForLookup(ip string) (dbprovider.Record, error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return dbprovider.Record{Country: ip}, fmt.Errorf("unable to parse IP address from [%s]", ip)
	}
	if ipAddr.IsPrivate() || ipAddr.IsLoopback() {
		return dbprovider.Record{Country: PrivateIpCountryAlias}, nil
	}
	rec, err := p.Lookup(ip)
	if err != nil {
		return dbprovider.Record{Country: ip}, fmt.Errorf("lookup of %s failed: %w", ip, err)
	}
	return rec, nil
}

// decide applies private, CIDR, and country maps using the given country (from countryHeader).
func (p Plugin) decide(ip, country string) (allow bool, rec dbprovider.Record, phase string, err error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false, dbprovider.Record{Country: ip}, "", fmt.Errorf("unable to parse IP address from [%s]", ip)
	}
	rec = dbprovider.Record{Country: country}
	if ipAddr.IsPrivate() || ipAddr.IsLoopback() {
		private := dbprovider.Record{Country: PrivateIpCountryAlias}
		if p.allowPrivate {
			return true, private, PhaseAllowPrivate, nil
		}
		return false, private, PhaseAllowPrivate, nil
	}

	blocked, blockedNetworkLength, err := p.isBlockedIPBlocks(ipAddr)
	if err != nil {
		return false, rec, "", fmt.Errorf("failed to check if IP %q is blocked by IP block: %w", ip, err)
	}

	allowed, allowedNetworkLength, err := p.isAllowedIPBlocks(ipAddr)
	if err != nil {
		return false, rec, "", fmt.Errorf("failed to check if IP %q is allowed by IP block: %w", ip, err)
	}

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

	if country == PrivateIpCountryAlias {
		private := dbprovider.Record{Country: PrivateIpCountryAlias}
		if p.allowPrivate {
			return true, private, PhaseAllowPrivate, nil
		}
		return false, private, PhaseAllowPrivate, nil
	}

	if _, ok := p.allowedCountries[country]; ok {
		return true, rec, PhaseAllowedCountry, nil
	}
	if _, ok := p.blockedCountries[country]; ok {
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
func (p Plugin) serveBanHtml(rw http.ResponseWriter, ip, country, requestMethod string) {
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
