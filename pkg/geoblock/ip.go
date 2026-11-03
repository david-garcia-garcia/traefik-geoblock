package geoblock

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup"
)

// GetRemoteIPs is first-line Header.Get plus first-occurrence dedup for CheckAll / CheckFirst / CheckFirstNonePrivate.
func (p Plugin) GetRemoteIPs(req *http.Request) []string {
	return dedupeFirst(collectHops(req, p.ipHeaders, false))
}

// collectHops reads the named headers, splits on ',', runs cleanIPAddress, and does not dedupe.
// allLines true uses every field line (Header.Values); false uses the first line (Header.Get).
func collectHops(req *http.Request, names []string, allLines bool) []string {
	var hops []string
	for _, headerName := range names {
		if headerName == remoteAddressHeader {
			ip := cleanIPAddress(req.RemoteAddr)
			if ip != "" {
				hops = append(hops, ip)
			}
			continue
		}
		for _, line := range headerLines(req, headerName, allLines) {
			hops = append(hops, splitHops(line)...)
		}
	}
	return hops
}

// headerLines is the field lines of one header, or nothing when the header is absent.
func headerLines(req *http.Request, headerName string, allLines bool) []string {
	if allLines {
		return req.Header.Values(headerName)
	}
	value := req.Header.Get(headerName)
	if value == "" {
		return nil
	}
	return []string{value}
}

// splitHops is the comma-separated hops in one field line after cleanIPAddress.
func splitHops(headerValue string) []string {
	var hops []string
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
		hops = append(hops, ip)
	}
	return hops
}

// dedupeFirst keeps the first occurrence of each hop and drops later repeats.
func dedupeFirst(hops []string) []string {
	if len(hops) < 2 {
		return hops
	}
	var ips []string
	var seenIPs map[string]struct{}
	for _, ip := range hops {
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
	return ips
}

// rightmostChain is the first present forwarding header plus RemoteAddr when remoteAddress is listed.
func (p Plugin) rightmostChain(req *http.Request) []string {
	var hops []string
	hasRemoteAddress := false
	for _, headerName := range p.ipHeaders {
		if headerName == remoteAddressHeader {
			hasRemoteAddress = true
			continue
		}
		if len(hops) > 0 {
			continue
		}
		hops = collectHops(req, []string{headerName}, true)
	}
	if !hasRemoteAddress {
		return hops
	}
	ip := cleanIPAddress(req.RemoteAddr)
	if ip == "" {
		return hops
	}
	return append(hops, ip)
}

// rightmostUntrusted walks hops from the right and returns the first hop that is not a trusted proxy.
func (p Plugin) rightmostUntrusted(hops []string) []string {
	for i := len(hops) - 1; i >= 0; i-- {
		if p.trustedHop(hops[i]) {
			continue
		}
		return []string{hops[i]}
	}
	return nil
}

// trustedHop reports whether hop is in trustedProxies. Unparsable and Contains errors are not trusted.
func (p Plugin) trustedHop(hop string) bool {
	if p.trustedProxies == nil {
		return false
	}
	ipAddr := net.ParseIP(hop)
	if ipAddr == nil {
		return false
	}
	found, _, _, err := p.trustedProxies.Contains(ipAddr)
	if err != nil {
		return false
	}
	return found
}

// newTrustedProxyHelper stores trustedProxies on a family-isolated helper. A plain address becomes /32 or /128.
func newTrustedProxyHelper(entries []string) (*iplookup.Helper, error) {
	helper := iplookup.New()
	for _, entry := range entries {
		cidr := strings.TrimSpace(entry)
		if cidr == "" {
			continue
		}
		if !strings.Contains(cidr, "/") {
			ipAddr := net.ParseIP(cidr)
			if ipAddr == nil {
				return nil, fmt.Errorf("trustedProxies: invalid entry %q", entry)
			}
			if ipAddr.To4() != nil {
				cidr = cidr + "/32"
			} else {
				cidr = cidr + "/128"
			}
		}
		if err := helper.AddCIDR(cidr, ""); err != nil {
			return nil, fmt.Errorf("trustedProxies: invalid entry %q: %w", entry, err)
		}
	}
	return helper, nil
}

// canonicalIPHeaders canonicalizes configured IP header names. remoteAddress stays literal.
func canonicalIPHeaders(headers []string) []string {
	out := make([]string, len(headers))
	for i, headerName := range headers {
		if headerName == remoteAddressHeader {
			out[i] = headerName
			continue
		}
		out[i] = http.CanonicalHeaderKey(headerName)
	}
	return out
}

// firstIP is the first extracted IP, or empty.
func firstIP(ips []string) string {
	if len(ips) == 0 {
		return ""
	}
	return ips[0]
}

// privateOrLoopback reports whether ip is RFC1918/ULA or loopback.
func privateOrLoopback(ip string) bool {
	ipAddr := net.ParseIP(ip)
	return ipAddr != nil && (ipAddr.IsPrivate() || ipAddr.IsLoopback())
}

// cleanIPAddress trims a hop and strips host:port when present.
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
