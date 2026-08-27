package ipinfo

import (
	"strings"
	"time"
)

const (
	// DefaultCode is IPinfo Lite (free MMDB). Official slugs: lite, core, plus.
	DefaultCode = "lite"

	// DefaultFileName is the committed Lite snapshot at the module root.
	DefaultFileName = "ipinfo_" + DefaultCode + ".mmdb"
)

func normalizeCode(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" {
		return DefaultCode
	}
	return c
}

func knownPackageCode(code string) bool {
	switch normalizeCode(code) {
	case "lite", "core", "plus":
		return true
	default:
		return false
	}
}

func fileNameForCode(code string) string {
	return "ipinfo_" + normalizeCode(code) + ".mmdb"
}

func maxDownloadBytesFor(code string) int64 {
	switch normalizeCode(code) {
	case "plus":
		return 6 * 1024 * 1024 * 1024
	case "core":
		return 1024 * 1024 * 1024
	default:
		return 64 * 1024 * 1024
	}
}

func downloadTimeoutFor(code string) time.Duration {
	switch normalizeCode(code) {
	case "plus":
		return 30 * time.Minute
	case "core":
		return 10 * time.Minute
	default:
		return 2 * time.Minute
	}
}
