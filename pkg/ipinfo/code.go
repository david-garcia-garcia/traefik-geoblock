package ipinfo

import (
	"strings"
	"time"
)

const (
	CodeLite = "lite"
	CodeCore = "core"
	CodePlus = "plus"

	// DefaultCode is IPinfo Lite (free MMDB). Official slugs: lite, core, plus.
	DefaultCode = CodeLite

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
	case CodeLite, CodeCore, CodePlus:
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
	case CodePlus:
		return 6 * 1024 * 1024 * 1024
	case CodeCore:
		return 1024 * 1024 * 1024
	default:
		return 64 * 1024 * 1024
	}
}

func downloadTimeoutFor(code string) time.Duration {
	switch normalizeCode(code) {
	case CodePlus:
		return 30 * time.Minute
	case CodeCore:
		return 10 * time.Minute
	default:
		return 2 * time.Minute
	}
}
