package maxmind

import (
	"strings"
	"time"
)

const (
	CodeGeoLite2Country = "GeoLite2-Country"
	CodeGeoLite2City    = "GeoLite2-City"
	CodeGeoIP2Country   = "GeoIP2-Country"
	CodeGeoIP2City      = "GeoIP2-City"

	DefaultCode = CodeGeoLite2Country

	// DefaultSeedFileName is MaxMind's official dummy Country fixture at the module root.
	DefaultSeedFileName = "GeoIP2-Country-Test.mmdb"
)

func normalizeCode(code string) string {
	c := strings.TrimSpace(code)
	if c == "" {
		return DefaultCode
	}
	return c
}

func knownEdition(code string) bool {
	switch normalizeCode(code) {
	case CodeGeoLite2Country, CodeGeoLite2City, CodeGeoIP2Country, CodeGeoIP2City:
		return true
	default:
		return false
	}
}

func fileNameForCode(code string) string {
	return normalizeCode(code) + ".mmdb"
}

func maxDownloadBytes() int64 {
	return 512 * 1024 * 1024
}

func downloadTimeout() time.Duration {
	return 10 * time.Minute
}
