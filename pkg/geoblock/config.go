package geoblock

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/ip2location"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/maxmind"
)

const (
	DatabaseProviderIP2Location = "ip2location"
	DatabaseProviderIPinfo      = "ipinfo"
	DatabaseProviderMaxMind     = "maxmind"

	// DefaultIP2LocationCatalogKey is the reserved catalog name for the free geo LITE ZIP.
	DefaultIP2LocationCatalogKey = "default_ip2location"
	// DefaultGeoliteCatalogKey is the reserved catalog name for the unofficial Country MMDB GET.
	DefaultGeoliteCatalogKey = "default_geolite"
	defaultAutoUpdateDirName = "traefik-geoblock"
)

// IP header strategy constants
const (
	IPHeaderStrategyCheckAll              = "CheckAll"
	IPHeaderStrategyCheckFirst            = "CheckFirst"
	IPHeaderStrategyCheckFirstNonePrivate = "CheckFirstNonePrivate"
)

// Mode is the Traefik Config field that selects lookup, block, both, or pass-through.
const (
	ModeDisabled       = "disabled"
	ModeEnrich         = "enrich"
	ModeBlock          = "block"
	ModeEnrichAndBlock = "enrichandblock"

	// DefaultCountryHeader is used when countryHeader is omitted.
	DefaultCountryHeader = "X-IPCountry"
)

// Config defines the plugin configuration.
type Config struct {
	// Mode is disabled, enrich, block, or enrichandblock. Empty is disabled.
	Mode         string `json:"mode,omitempty" mapstructure:"mode"`
	DefaultAllow bool   // Default behavior when IP matches no rules
	AllowPrivate bool   // Allow requests from private/internal networks
	BanIfError   bool   // Ban requests if IP lookup fails

	// Database provider. Empty defaults to ip2location. Implemented: ip2location, ipinfo, maxmind.
	DatabaseProvider string `json:"databaseProvider,omitempty" mapstructure:"databaseProvider"`

	// DatabaseSources is the catalog of database files (seed path and/or URL).
	// Operator keys plus reserved default_ip2location and default_geolite.
	DatabaseSources map[string]DatabaseSource `json:"databaseSources,omitempty" mapstructure:"databaseSources"`
	// DatabaseAutoUpdateDir is the shared dir for dated files. Empty with a bound URL uses a temp dir.
	DatabaseAutoUpdateDir string `json:"databaseAutoUpdateDir,omitempty" mapstructure:"databaseAutoUpdateDir"`

	// Catalog pointers. Empty IP2Location geo binds default_ip2location.
	// Empty MaxMind binds default_geolite. Unused pointers are ignored.
	Ip2locationSourceGeo string `json:"ip2location_source_geo,omitempty" mapstructure:"ip2location_source_geo"`
	Ip2locationSourceAsn string `json:"ip2location_source_asn,omitempty" mapstructure:"ip2location_source_asn"`
	IpinfoSource         string `json:"ipinfo_source,omitempty" mapstructure:"ipinfo_source"`
	MaxmindSource        string `json:"maxmind_source,omitempty" mapstructure:"maxmind_source"`

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
	CountryHeader         string // Lookup writes; block reads. Empty becomes DefaultCountryHeader.
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

// DatabaseSource is one catalog row (seed path and/or GET URL).
type DatabaseSource struct {
	URL          string            `json:"url,omitempty" mapstructure:"url"`
	Headers      map[string]string `json:"headers,omitempty" mapstructure:"headers"`
	DatabaseType string            `json:"databaseType,omitempty" mapstructure:"databaseType"`
	Archive      string            `json:"archive,omitempty" mapstructure:"archive"`
	Path         string            `json:"path,omitempty" mapstructure:"path"`
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
		CountryHeader:        DefaultCountryHeader,
		DatabaseSources:      make(map[string]DatabaseSource),
	}
}

// NormalizeMode is empty → disabled, else lowercased trimmed mode.
func NormalizeMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		return ModeDisabled
	}
	return m
}

// ModeLooksUp reports whether this mode opens the DatabaseProvider and writes headers.
func ModeLooksUp(mode string) bool {
	return mode == ModeEnrich || mode == ModeEnrichAndBlock
}

// ModeBlocks reports whether this mode applies CIDR, private, and country rules.
func ModeBlocks(mode string) bool {
	return mode == ModeBlock || mode == ModeEnrichAndBlock
}

// validMode reports whether mode is one of the four allowed values.
func validMode(mode string) bool {
	switch mode {
	case ModeDisabled, ModeEnrich, ModeBlock, ModeEnrichAndBlock:
		return true
	default:
		return false
	}
}

// Prepare normalizes and validates cfg. Mutates cfg. Call before hashing or NewCore.
func Prepare(cfg *Config, name string) error {
	cfg.Mode = NormalizeMode(cfg.Mode)
	if cfg.Mode == ModeDisabled {
		return nil
	}
	if !validMode(cfg.Mode) {
		return fmt.Errorf("%s: invalid mode %q, must be one of: %s, %s, %s, %s",
			name, cfg.Mode, ModeDisabled, ModeEnrich, ModeBlock, ModeEnrichAndBlock)
	}
	logger := pluginLogger(name, cfg)
	if http.StatusText(cfg.DisallowedStatusCode) == "" {
		return fmt.Errorf("%s: %d is not a valid http status code", name, cfg.DisallowedStatusCode)
	}
	if len(cfg.IPHeaders) == 0 {
		return fmt.Errorf("%s: IPHeaders cannot be empty - at least one header must be specified for IP extraction", name)
	}
	if cfg.IPHeaderStrategy != IPHeaderStrategyCheckAll &&
		cfg.IPHeaderStrategy != IPHeaderStrategyCheckFirst &&
		cfg.IPHeaderStrategy != IPHeaderStrategyCheckFirstNonePrivate {
		return fmt.Errorf("%s: invalid IPHeaderStrategy '%s', must be one of: %s, %s, %s",
			name, cfg.IPHeaderStrategy,
			IPHeaderStrategyCheckAll, IPHeaderStrategyCheckFirst, IPHeaderStrategyCheckFirstNonePrivate)
	}
	if strings.TrimSpace(cfg.CountryHeader) == "" {
		cfg.CountryHeader = DefaultCountryHeader
	}
	if err := checkCountryHeaderBridge(cfg.CountryHeader, cfg.RequestHeaderEnrich); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	if ModeLooksUp(cfg.Mode) {
		ensureDefaultIP2LocationCatalog(cfg)
		ensureDefaultGeoliteCatalog(cfg)
		applyMissingPointerFallbacks(cfg, logger)
		bindEmptyIP2LocationGeo(cfg)
		bindEmptyMaxmindSource(cfg)
		if err := validateDatabaseSources(cfg); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		applyTempAutoUpdateDir(cfg, logger)
	}

	if cfg.BanHtmlFilePath != "" {
		var err error
		cfg.BanHtmlFilePath, err = fileutils.Default.Search(cfg.BanHtmlFilePath, "geoblockban.html", logger)
		if err != nil {
			return fmt.Errorf("%s: failed to find ban HTML file: %w", name, err)
		}
	}
	return nil
}

// checkCountryHeaderBridge fails when requestHeaderEnrich maps a different header to country.
func checkCountryHeaderBridge(countryHeader string, enrich map[string]string) error {
	canon := http.CanonicalHeaderKey(strings.TrimSpace(countryHeader))
	for header, key := range enrich {
		if strings.ToLower(strings.TrimSpace(key)) != dbprovider.MetaCountry {
			continue
		}
		if http.CanonicalHeaderKey(header) != canon {
			return fmt.Errorf("requestHeaderEnrich header %q maps to country; countryHeader is %q", header, countryHeader)
		}
	}
	return nil
}

// providerName is the selected DatabaseProvider, or ip2location when empty.
func providerName(cfg *Config) string {
	name := strings.ToLower(strings.TrimSpace(cfg.DatabaseProvider))
	if name == "" {
		return DatabaseProviderIP2Location
	}
	return name
}

// boundSourceKeys is the catalog keys the selected provider will open.
func boundSourceKeys(cfg *Config) []string {
	switch providerName(cfg) {
	case DatabaseProviderIP2Location:
		return []string{cfg.Ip2locationSourceGeo, cfg.Ip2locationSourceAsn}
	case DatabaseProviderIPinfo:
		return []string{cfg.IpinfoSource}
	case DatabaseProviderMaxMind:
		return []string{cfg.MaxmindSource}
	default:
		return nil
	}
}

// catalogSource builds a dbsource.Config from one catalog key.
func catalogSource(cfg *Config, key, databaseType string) dbsource.Config {
	key = strings.TrimSpace(key)
	if key == "" {
		return dbsource.Config{}
	}
	var entry DatabaseSource
	if cfg.DatabaseSources != nil {
		entry = cfg.DatabaseSources[key]
	}
	return dbsource.Config{
		Key:          key,
		URL:          strings.TrimSpace(entry.URL),
		Path:         strings.TrimSpace(entry.Path),
		Headers:      entry.Headers,
		DatabaseType: firstNonEmpty(entry.DatabaseType, databaseType),
		Archive:      entry.Archive,
		Dir:          strings.TrimSpace(cfg.DatabaseAutoUpdateDir),
		MinAge:       dbsource.DefaultMinAge,
	}
}

// firstNonEmpty returns the first non-blank value.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ensureDefaultIP2LocationCatalog inserts the reserved free LITE row unless the operator already set that key.
func ensureDefaultIP2LocationCatalog(cfg *Config) {
	if cfg.DatabaseSources == nil {
		cfg.DatabaseSources = make(map[string]DatabaseSource)
	}
	if _, ok := cfg.DatabaseSources[DefaultIP2LocationCatalogKey]; ok {
		return
	}
	cfg.DatabaseSources[DefaultIP2LocationCatalogKey] = DatabaseSource{
		URL:          ip2location.DefaultLiteURL,
		DatabaseType: dbsource.TypeBIN,
		Archive:      dbsource.ArchiveZIP,
	}
}

// ensureDefaultGeoliteCatalog inserts the reserved Country MMDB row unless the operator already set that key.
func ensureDefaultGeoliteCatalog(cfg *Config) {
	if cfg.DatabaseSources == nil {
		cfg.DatabaseSources = make(map[string]DatabaseSource)
	}
	if _, ok := cfg.DatabaseSources[DefaultGeoliteCatalogKey]; ok {
		return
	}
	cfg.DatabaseSources[DefaultGeoliteCatalogKey] = DatabaseSource{
		URL:          maxmind.DefaultGeoliteURL,
		DatabaseType: dbsource.TypeMMDB,
		Archive:      dbsource.ArchiveNone,
	}
}

// applyMissingPointerFallbacks clears a bound pointer that is not a catalog key and WARNs.
func applyMissingPointerFallbacks(cfg *Config, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	switch providerName(cfg) {
	case DatabaseProviderIP2Location:
		cfg.Ip2locationSourceGeo = fallbackPointer(cfg, "ip2location_source_geo", cfg.Ip2locationSourceGeo, logger)
		cfg.Ip2locationSourceAsn = fallbackPointer(cfg, "ip2location_source_asn", cfg.Ip2locationSourceAsn, logger)
	case DatabaseProviderIPinfo:
		cfg.IpinfoSource = fallbackPointer(cfg, "ipinfo_source", cfg.IpinfoSource, logger)
	case DatabaseProviderMaxMind:
		cfg.MaxmindSource = fallbackPointer(cfg, "maxmind_source", cfg.MaxmindSource, logger)
	}
}

// fallbackPointer returns key when it names a catalog row, otherwise "" after a WARN.
func fallbackPointer(cfg *Config, field, key string, logger *slog.Logger) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if cfg.DatabaseSources != nil {
		if _, ok := cfg.DatabaseSources[key]; ok {
			return key
		}
	}
	logger.Warn("source pointer is not a key in databaseSources; using default", "pointer", field, "key", key)
	return ""
}

// bindEmptyIP2LocationGeo sets an empty IP2Location geo pointer to the reserved catalog key.
func bindEmptyIP2LocationGeo(cfg *Config) {
	if providerName(cfg) != DatabaseProviderIP2Location {
		return
	}
	if strings.TrimSpace(cfg.Ip2locationSourceGeo) == "" {
		cfg.Ip2locationSourceGeo = DefaultIP2LocationCatalogKey
	}
}

// bindEmptyMaxmindSource sets an empty MaxMind pointer to the reserved catalog key.
func bindEmptyMaxmindSource(cfg *Config) {
	if providerName(cfg) != DatabaseProviderMaxMind {
		return
	}
	if strings.TrimSpace(cfg.MaxmindSource) == "" {
		cfg.MaxmindSource = DefaultGeoliteCatalogKey
	}
}

// applyTempAutoUpdateDir sets a process temp dir when a bound URL has no operator dir.
func applyTempAutoUpdateDir(cfg *Config, logger *slog.Logger) {
	if strings.TrimSpace(cfg.DatabaseAutoUpdateDir) != "" {
		return
	}
	if !boundURLNeedsDir(cfg) {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	dir := filepath.Join(os.TempDir(), defaultAutoUpdateDirName)
	logger.Warn("databaseAutoUpdateDir is empty; using temp dir", "dir", dir)
	cfg.DatabaseAutoUpdateDir = dir
}

// boundURLNeedsDir reports whether a selected-provider pointer names a catalog row with a URL.
func boundURLNeedsDir(cfg *Config) bool {
	for _, key := range boundSourceKeys(cfg) {
		key = strings.TrimSpace(key)
		if key == "" || cfg.DatabaseSources == nil {
			continue
		}
		entry, ok := cfg.DatabaseSources[key]
		if ok && strings.TrimSpace(entry.URL) != "" {
			return true
		}
	}
	return false
}

// validateDatabaseSources normalizes each catalog row and checks bound pointer types.
func validateDatabaseSources(cfg *Config) error {
	for name, entry := range cfg.DatabaseSources {
		c := dbsource.Config{
			Key:          name,
			URL:          entry.URL,
			DatabaseType: entry.DatabaseType,
			Archive:      entry.Archive,
		}
		if err := dbsource.Normalize(&c); err != nil {
			return fmt.Errorf("databaseSources.%s: %w", name, err)
		}
	}
	return checkBoundPointerTypes(cfg)
}

// checkBoundPointerTypes fails when a bound catalog row's databaseType does not match the provider.
func checkBoundPointerTypes(cfg *Config) error {
	want := providerDatabaseType(cfg)
	if want == "" {
		return nil
	}
	for _, key := range boundSourceKeys(cfg) {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		entry, ok := cfg.DatabaseSources[key]
		if !ok {
			continue
		}
		got := strings.ToLower(strings.TrimSpace(entry.DatabaseType))
		if got != "" && got != want {
			return fmt.Errorf("source pointer %q has databaseType %q; %s requires %s", key, got, providerName(cfg), want)
		}
	}
	return nil
}

// providerDatabaseType is the catalog databaseType the selected provider can open.
func providerDatabaseType(cfg *Config) string {
	switch providerName(cfg) {
	case DatabaseProviderIP2Location:
		return dbsource.TypeBIN
	case DatabaseProviderIPinfo, DatabaseProviderMaxMind:
		return dbsource.TypeMMDB
	default:
		return ""
	}
}

// normalizeRequestHeaderEnrich canonicalizes header names and checks metadata keys.
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

// foldCountryHeader copies countryHeader into requestHeaderEnrich as key country
// so lookup writes the bridge header. A set enrich mapping for the same name wins.
func foldCountryHeader(countryHeader string, enrich map[string]string) map[string]string {
	h := strings.TrimSpace(countryHeader)
	if h == "" {
		return enrich
	}
	canon := http.CanonicalHeaderKey(h)
	if enrich == nil {
		enrich = map[string]string{}
	}
	if _, exists := enrich[canon]; !exists {
		enrich[canon] = dbprovider.MetaCountry
	}
	return enrich
}
