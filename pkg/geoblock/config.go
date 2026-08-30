package geoblock

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

const (
	// DefaultIP2LocationCatalogKey is the reserved enabled geo LITE ZIP + seed.
	DefaultIP2LocationCatalogKey = "default_ip2location"
	// DefaultIPinfoCatalogKey is the reserved disabled IPinfo Lite seed.
	DefaultIPinfoCatalogKey = "default_ipinfo"
	// DefaultMaxmindCatalogKey is the reserved disabled dummy Country seed.
	DefaultMaxmindCatalogKey = "default_maxmind"
	// DefaultGeoliteCatalogKey is the reserved disabled unofficial Country GET.
	DefaultGeoliteCatalogKey = "default_geolite"
	defaultAutoUpdateDirName = "traefik-geoblock"

	// DefaultIP2LocationLiteURL is the official free IP2Location geo LITE ZIP (no token).
	DefaultIP2LocationLiteURL = "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP"
	// DefaultIP2LocationGeoFile is the committed country LITE BIN under seeds/.
	DefaultIP2LocationGeoFile = "IP2LOCATION-LITE-DB1.IPV6.BIN"
	// DefaultIPinfoFile is the committed Lite snapshot under seeds/.
	DefaultIPinfoFile = "ipinfo_lite.mmdb"
	// DefaultMaxMindSeedFile is MaxMind's official dummy Country fixture under seeds/.
	DefaultMaxMindSeedFile = "GeoIP2-Country-Test.mmdb"
	// DefaultGeoliteURL is the unofficial P3TERX Country MMDB on the download branch.
	DefaultGeoliteURL = "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"
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

	// DatabaseSources is the catalog of lookup sources (path, URL, databaseType, defaultFile, fields, enabled).
	DatabaseSources map[string]DatabaseSource `json:"databaseSources,omitempty" mapstructure:"databaseSources"`
	// DatabaseAutoUpdateDir is the shared dir for dated files. Empty with an enabled URL uses a temp dir.
	DatabaseAutoUpdateDir string `json:"databaseAutoUpdateDir,omitempty" mapstructure:"databaseAutoUpdateDir"`

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

// DatabaseSource is one catalog row: file location, format, optional fields allowlist, and enabled.
type DatabaseSource struct {
	URL          string            `json:"url,omitempty" mapstructure:"url"`
	Headers      map[string]string `json:"headers,omitempty" mapstructure:"headers"`
	DatabaseType string            `json:"databaseType,omitempty" mapstructure:"databaseType"`
	Archive      string            `json:"archive,omitempty" mapstructure:"archive"`
	Path         string            `json:"path,omitempty" mapstructure:"path"`
	DefaultFile  string            `json:"defaultFile,omitempty" mapstructure:"defaultFile"`
	Enabled      *bool             `json:"enabled,omitempty" mapstructure:"enabled"`
	// Fields is the Record keys this row may fill (country, asn, …). Empty keeps every mapped key.
	Fields []string `json:"fields,omitempty" mapstructure:"fields"`
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

// ModeLooksUp reports whether this mode opens catalog sources and writes headers.
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
		insertReservedCatalog(cfg)
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

// sourceEnabled is true when enabled is omitted or true.
func sourceEnabled(entry DatabaseSource) bool {
	return entry.Enabled == nil || *entry.Enabled
}

// boolPtr is a *bool for catalog Enabled literals.
func boolPtr(v bool) *bool {
	return &v
}

// enabledSourceKeys is enabled catalog keys in lexicographic order.
func enabledSourceKeys(cfg *Config) []string {
	if cfg.DatabaseSources == nil {
		return nil
	}
	var keys []string
	for name, entry := range cfg.DatabaseSources {
		if sourceEnabled(entry) {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	return keys
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
		Key:             key,
		URL:             strings.TrimSpace(entry.URL),
		Path:            strings.TrimSpace(entry.Path),
		Headers:         entry.Headers,
		DatabaseType:    firstNonEmpty(entry.DatabaseType, databaseType),
		Archive:         entry.Archive,
		Dir:             strings.TrimSpace(cfg.DatabaseAutoUpdateDir),
		MinAge:          dbsource.DefaultMinAge,
		DefaultFileName: strings.TrimSpace(entry.DefaultFile),
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

// insertReservedCatalog adds shipped seed and download rows when the key is absent.
func insertReservedCatalog(cfg *Config) {
	if cfg.DatabaseSources == nil {
		cfg.DatabaseSources = make(map[string]DatabaseSource)
	}
	insertCatalogIfAbsent(cfg, DefaultIP2LocationCatalogKey, DatabaseSource{
		URL:          DefaultIP2LocationLiteURL,
		DatabaseType: dbsource.TypeBIN,
		Archive:      dbsource.ArchiveZIP,
		DefaultFile:  DefaultIP2LocationGeoFile,
	})
	insertCatalogIfAbsent(cfg, DefaultIPinfoCatalogKey, DatabaseSource{
		DatabaseType: dbsource.TypeMMDB,
		DefaultFile:  DefaultIPinfoFile,
		Enabled:      boolPtr(false),
	})
	insertCatalogIfAbsent(cfg, DefaultMaxmindCatalogKey, DatabaseSource{
		DatabaseType: dbsource.TypeMMDB,
		DefaultFile:  DefaultMaxMindSeedFile,
		Enabled:      boolPtr(false),
	})
	insertCatalogIfAbsent(cfg, DefaultGeoliteCatalogKey, DatabaseSource{
		URL:          DefaultGeoliteURL,
		DatabaseType: dbsource.TypeMMDB,
		Archive:      dbsource.ArchiveNone,
		Enabled:      boolPtr(false),
	})
}

// insertCatalogIfAbsent writes row when name is not already in the catalog.
func insertCatalogIfAbsent(cfg *Config, name string, row DatabaseSource) {
	if _, ok := cfg.DatabaseSources[name]; ok {
		return
	}
	cfg.DatabaseSources[name] = row
}

// applyTempAutoUpdateDir sets a process temp dir when an enabled URL has no operator dir.
func applyTempAutoUpdateDir(cfg *Config, logger *slog.Logger) {
	if strings.TrimSpace(cfg.DatabaseAutoUpdateDir) != "" {
		return
	}
	if !enabledURLNeedsDir(cfg) {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	dir := filepath.Join(os.TempDir(), defaultAutoUpdateDirName)
	logger.Warn("databaseAutoUpdateDir is empty; using temp dir", "dir", dir)
	cfg.DatabaseAutoUpdateDir = dir
}

// enabledURLNeedsDir reports whether an enabled catalog row has a URL.
func enabledURLNeedsDir(cfg *Config) bool {
	for _, key := range enabledSourceKeys(cfg) {
		entry := cfg.DatabaseSources[key]
		if strings.TrimSpace(entry.URL) != "" {
			return true
		}
	}
	return false
}

// validateDatabaseSources normalizes rows and checks enabled format/fields.
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
	enabled := enabledSourceKeys(cfg)
	if len(enabled) == 0 {
		return fmt.Errorf("no enabled databaseSources")
	}
	for _, key := range enabled {
		entry := cfg.DatabaseSources[key]
		databaseType := strings.ToLower(strings.TrimSpace(entry.DatabaseType))
		if databaseType != dbsource.TypeBIN && databaseType != dbsource.TypeMMDB {
			return fmt.Errorf("databaseSources.%s: unknown or empty databaseType %q", key, entry.DatabaseType)
		}
		fields, err := normalizeSourceFields(entry.Fields)
		if err != nil {
			return fmt.Errorf("databaseSources.%s: %w", key, err)
		}
		entry.DatabaseType = databaseType
		entry.Fields = fields
		cfg.DatabaseSources[key] = entry
	}
	return nil
}

// normalizeSourceFields lowercases Record keys and rejects unknown names.
func normalizeSourceFields(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" {
			continue
		}
		if !dbprovider.KnownMetaKey(key) {
			return nil, fmt.Errorf("unknown fields value %q (supported: %s)",
				raw, strings.Join(dbprovider.MetaKeys(), ", "))
		}
		out = append(out, key)
	}
	return out, nil
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
