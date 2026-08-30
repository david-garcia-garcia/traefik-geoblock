package geoblock

import (
	"fmt"
	"regexp"
)

// compilePathRegex compiles an include/exclude path pattern. Empty pattern is unset.
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

// pathRegexMatches is true when re matches host+path.
func pathRegexMatches(re *regexp.Regexp, host, path string) bool {
	if re == nil {
		return false
	}
	return re.MatchString(host + path)
}
