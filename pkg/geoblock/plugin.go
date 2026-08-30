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
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
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

// Plugin is the shared policy for one middleware name and config, including catalog lookup.
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

// NewCore builds the Plugin and opens catalog sources on this incarnation’s lifetime. cfg must already be Prepare'd.
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

// bindDatabase opens enabled catalog sources on this incarnation’s lifetime.
func (p *Plugin) bindDatabase(life context.Context, cfg *Config) error {
	if !ModeLooksUp(p.mode) {
		return nil
	}
	db, err := openCatalogSources(life, cfg, logging.NewBootstrap(p.name, cfg.LogLevel))
	if err != nil {
		return fmt.Errorf("%s: failed to open catalog sources: %w", p.name, err)
	}
	p.db = db
	return nil
}

// openCatalogSources opens each enabled row and returns a Combined Lookup.
func openCatalogSources(ctx context.Context, cfg *Config, logger *slog.Logger) (dbprovider.Provider, error) {
	var sources []dbprovider.Named
	for _, key := range enabledSourceKeys(cfg) {
		entry := cfg.DatabaseSources[key]
		vendor := strings.ToLower(strings.TrimSpace(entry.Vendor))
		lookup, err := openCatalogRow(ctx, cfg, key, vendor, logger)
		if err != nil {
			return nil, err
		}
		sources = append(sources, dbprovider.Named{Key: key, Provider: lookup})
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no enabled databaseSources")
	}
	return dbprovider.NewCombined(sources), nil
}

// openCatalogRow opens one wrapper Lookup for a catalog key.
func openCatalogRow(ctx context.Context, cfg *Config, key, vendor string, logger *slog.Logger) (dbprovider.Provider, error) {
	entry := cfg.DatabaseSources[key]
	switch vendor {
	case VendorIP2Location:
		src := catalogSource(cfg, key, dbsource.TypeBIN)
		bin, err := dbwrappers.OpenBIN(ctx, dbwrappers.BINConfig{
			Dir:             cfg.DatabaseAutoUpdateDir,
			Source:          src,
			DefaultFileName: src.DefaultFileName,
			AllowMissing:    src.Path == "" && src.DefaultFileName == "",
			MinAge:          dbwrappers.DefaultBINMinAge,
		}, logger)
		if err != nil {
			return nil, err
		}
		return dbwrappers.NewBINSource(bin, entry.Fields), nil
	case VendorIPinfo:
		src := catalogSource(cfg, key, dbsource.TypeMMDB)
		mmdb, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{
			Dir:             cfg.DatabaseAutoUpdateDir,
			Source:          src,
			DefaultFileName: src.DefaultFileName,
			MinAge:          dbsource.DefaultMinAge,
		}, logger)
		if err != nil {
			return nil, err
		}
		return dbwrappers.NewIPinfo(mmdb, entry.Fields), nil
	case VendorMaxMind:
		src := catalogSource(cfg, key, dbsource.TypeMMDB)
		mmdb, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{
			Dir:             cfg.DatabaseAutoUpdateDir,
			Source:          src,
			DefaultFileName: src.DefaultFileName,
			MinAge:          dbsource.DefaultMinAge,
		}, logger)
		if err != nil {
			return nil, err
		}
		return dbwrappers.NewGeoIP2(mmdb, entry.Fields), nil
	default:
		return nil, fmt.Errorf("unknown vendor %q", vendor)
	}
}

// setDecisionLogHeader writes logStatusDetailHeader as "{status}:{reason}" when that header is configured.
func (p Plugin) setDecisionLogHeader(req *http.Request, status, reason string) {
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
	ipChain := p.traceIPChain(remoteIPs)

	// 1. Enrich: look up and write countryHeader / requestHeaderEnrich. Does not ban.
	lookupFailed := false
	if ModeLooksUp(p.mode) {
		lookupFailed = p.enrich(req, remoteIPs, ipChain)
	}

	// 2. Block: CIDR, private, country from countryHeader.
	if !ModeBlocks(p.mode) {
		next.ServeHTTP(rw, req)
		return
	}
	skipBlock := p.blockSkipReason(req, ipChain)
	if skipBlock != PhaseNone {
		p.setDecisionLogHeader(req, LogStatusPass, skipBlock)
		next.ServeHTTP(rw, req)
		return
	}
	if lookupFailed && p.banIfError {
		p.setDecisionLogHeader(req, LogStatusBlock, "error")
		p.serveBanHtml(rw, firstIP(remoteIPs), "Unknown", req.Method)
		return
	}
	if p.blockFromHeader(rw, req, remoteIPs, ipChain) {
		return
	}
	next.ServeHTTP(rw, req)
}

// traceIPChain is the joined hop list when trace logging is on.
func (p Plugin) traceIPChain(remoteIPs []string) string {
	if !p.logger.Enabled(context.Background(), logging.LevelTrace) {
		return ""
	}
	return strings.Join(remoteIPs, ", ")
}

// blockSkipReason is a verb / path / bypass match that skips the block stage. Enrich still runs.
func (p Plugin) blockSkipReason(req *http.Request, ipChain string) string {
	if _, ignored := p.ignoreVerbs[strings.ToUpper(req.Method)]; ignored {
		logging.Trace(p.logger, "HTTP verb ignored for blocking",
			"method", req.Method, "remote_addr", req.RemoteAddr, "ip_chain", ipChain)
		return PhaseIgnoreVerb
	}
	if !p.isIncludedByRegex(req.Host, req.URL.Path) {
		logging.Trace(p.logger, "request not included for blocking by regex",
			"host", req.Host, "path", req.URL.Path, "remote_addr", req.RemoteAddr, "ip_chain", ipChain)
		return PhaseNotIncludedRegex
	}
	if p.isExcludedByRegex(req.Host, req.URL.Path) {
		logging.Trace(p.logger, "request excluded from blocking by regex",
			"host", req.Host, "path", req.URL.Path, "remote_addr", req.RemoteAddr, "ip_chain", ipChain)
		return PhaseExcludedRegex
	}
	for header, expectedValue := range p.bypassHeaders {
		if actualValue := req.Header.Get(header); actualValue == expectedValue {
			logging.Trace(p.logger, "bypassing geoblock due to bypass header match",
				"header", header, "value", logging.Redact(expectedValue),
				"remote_addr", req.RemoteAddr, "ip_chain", ipChain)
			return PhaseBypassHeader
		}
	}
	return PhaseNone
}

// enrich writes countryHeader and requestHeaderEnrich. It does not ban.
// Defaults go on first so mapped headers exist; the first public lookup replaces them.
func (p Plugin) enrich(req *http.Request, remoteIPs []string, ipChain string) (lookupFailed bool) {
	var foundPublicIP bool
	var publicCountryWritten bool
	// Defaults first so every mapped header exists if no public lookup wins.
	p.writeDefaultEnrichHeaders(req)
	for i, ip := range remoteIPs {
		if p.skipIP(i, ip, remoteIPs, &foundPublicIP) {
			continue
		}
		rec, err := p.recordForLookup(ip)
		if !publicCountryWritten {
			p.writePublicLookupHeaders(req, rec, &publicCountryWritten)
		}
		if err != nil {
			p.logLookupError(req, ip, ipChain, remoteIPs, err)
			lookupFailed = true
			continue
		}
		if p.ipHeaderStrategy == IPHeaderStrategyCheckFirstNonePrivate && rec.Country != PrivateIpCountryAlias {
			break
		}
	}
	return lookupFailed
}

// blockFromHeader runs CIDR / private / country using countryHeader. true means banned.
func (p Plugin) blockFromHeader(rw http.ResponseWriter, req *http.Request, remoteIPs []string, ipChain string) bool {
	country := req.Header.Get(p.countryHeader)
	if country == "" || country == EnrichNullAlias {
		if p.banIfError {
			p.setDecisionLogHeader(req, LogStatusBlock, "error")
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
		allowed, phase, err := p.decide(ip, country)
		if err != nil {
			p.logLookupError(req, ip, ipChain, remoteIPs, err)
			if p.banIfError && passReason == PhaseNone {
				p.setDecisionLogHeader(req, LogStatusBlock, "error")
				p.serveBanHtml(rw, ip, "Unknown", req.Method)
				return true
			}
			continue
		}
		if !allowed && passReason == PhaseNone {
			p.setDecisionLogHeader(req, LogStatusBlock, phase)
			p.serveBanHtml(rw, ip, countryForBan(ip, country), req.Method)
			return true
		}
		if passReason == PhaseNone && allowed {
			passReason = phase
		}
		if p.ipHeaderStrategy == IPHeaderStrategyCheckFirstNonePrivate && !privateOrLoopback(ip) {
			break
		}
	}
	p.setDecisionLogHeader(req, LogStatusPass, passReason)
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
	isPrivate := privateOrLoopback(ip)
	if isPrivate && !*foundPublicIP && i < len(remoteIPs)-1 {
		return true
	}
	if !isPrivate {
		*foundPublicIP = true
	}
	return false
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

// CheckAllowed is private + CIDR + country from a lookup when catalog sources are bound.
// Request country rules use decide with countryHeader instead.
func (p Plugin) CheckAllowed(ip string) (allow bool, phase string, err error) {
	rec, err := p.recordForLookup(ip)
	if err != nil {
		return false, "", err
	}
	return p.decide(ip, rec.Country)
}

// recordForLookup is PRIVATE for private/loopback IPs, else catalog Lookup.
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

// decide applies private, CIDR, and country maps using country from countryHeader.
func (p Plugin) decide(ip, country string) (allow bool, phase string, err error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false, "", fmt.Errorf("unable to parse IP address from [%s]", ip)
	}
	// Private hops follow allowPrivate, not the inbound country.
	if ipAddr.IsPrivate() || ipAddr.IsLoopback() {
		return p.allowPrivate, PhaseAllowPrivate, nil
	}

	blocked, blockedNetworkLength, err := p.isBlockedIPBlocks(ipAddr)
	if err != nil {
		return false, "", fmt.Errorf("failed to check if IP %q is blocked by IP block: %w", ip, err)
	}

	allowed, allowedNetworkLength, err := p.isAllowedIPBlocks(ipAddr)
	if err != nil {
		return false, "", fmt.Errorf("failed to check if IP %q is allowed by IP block: %w", ip, err)
	}

	if (allowedNetworkLength < blockedNetworkLength) && (allowedNetworkLength > 0) && (blockedNetworkLength > 0) {
		if blocked {
			return false, PhaseBlockedIPBlock, nil
		}
		if allowed {
			return true, PhaseAllowedIPBlock, nil
		}
	} else {
		if allowed {
			return true, PhaseAllowedIPBlock, nil
		}
		if blocked {
			return false, PhaseBlockedIPBlock, nil
		}
	}

	if country == PrivateIpCountryAlias {
		return p.allowPrivate, PhaseAllowPrivate, nil
	}

	if _, ok := p.allowedCountries[country]; ok {
		return true, PhaseAllowedCountry, nil
	}
	if _, ok := p.blockedCountries[country]; ok {
		return false, PhaseBlockedCountry, nil
	}
	if p.defaultAllow {
		return true, PhaseDefaultAllow, nil
	}
	return false, PhaseDefaultAllow, nil
}

// countryForBan is PRIVATE for a private hop, else the countryHeader value.
func countryForBan(ip, country string) string {
	if privateOrLoopback(ip) {
		return PrivateIpCountryAlias
	}
	return country
}

// Lookup returns geo metadata for ip.
func (p Plugin) Lookup(ip string) (dbprovider.Record, error) {
	return p.db.Lookup(ip)
}

// writeDefaultEnrichHeaders puts a value on every requestHeaderEnrich mapping.
// Country is PRIVATE (not blank): a private-only chain still has a country for
// allowPrivate, and block will not treat the header as missing. Other keys are null.
func (p Plugin) writeDefaultEnrichHeaders(req *http.Request) {
	for header, key := range p.requestHeaderEnrich {
		if key == dbprovider.MetaCountry {
			req.Header.Set(header, PrivateIpCountryAlias)
			continue
		}
		req.Header.Set(header, EnrichNullAlias)
	}
}

// writePublicLookupHeaders copies rec onto the enrich headers when rec is a public
// country. Private or empty country is ignored so the defaults stay. written becomes
// true so a later hop cannot replace the first public country.
func (p Plugin) writePublicLookupHeaders(req *http.Request, rec dbprovider.Record, written *bool) {
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
	*written = true
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
