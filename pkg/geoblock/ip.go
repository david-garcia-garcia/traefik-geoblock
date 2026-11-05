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
	var ips []string
	var seen map[string]struct{}
	for _, headerName := range p.ipHeaders {
		value := req.RemoteAddr
		if headerName != remoteAddressHeader {
			value = req.Header.Get(headerName)
		}
		ips = appendHops(ips, value, &seen)
	}
	return ips
}

// rightmostChain is the RightmostUntrusted hop chain, left to right.
// Forwarding headers are tried in ipHeaders order and only the first one that yields a hop is used:
// an absent header, an empty value, or a value with no hop (" , ") is skipped. Every field line of that
// header is read in order. RemoteAddr closes the chain when remoteAddress is listed (Prepare keeps it last).
// Repeated hops are kept.
func (p Plugin) rightmostChain(req *http.Request) []string {
	var hops []string
	for _, headerName := range p.ipHeaders {
		if headerName == remoteAddressHeader {
			return appendHops(hops, req.RemoteAddr, nil)
		}
		if len(hops) > 0 {
			continue
		}
		for _, line := range req.Header.Values(headerName) {
			hops = appendHops(hops, line, nil)
		}
	}
	return hops
}

// appendHops appends the comma-separated hops of value after cleanIPAddress.
// A non-nil seen drops hops already appended (first occurrence wins); the map is made on the second hop.
func appendHops(dst []string, value string, seen *map[string]struct{}) []string {
	for len(value) > 0 {
		var part string
		if i := strings.IndexByte(value, ','); i >= 0 {
			part = value[:i]
			value = value[i+1:]
		} else {
			part = value
			value = ""
		}
		ip := cleanIPAddress(part)
		if ip == "" {
			continue
		}
		if seen != nil && len(dst) > 0 {
			if *seen == nil {
				*seen = make(map[string]struct{}, 2)
				(*seen)[dst[0]] = struct{}{}
			}
			if _, dup := (*seen)[ip]; dup {
				continue
			}
			(*seen)[ip] = struct{}{}
		}
		dst = append(dst, ip)
	}
	return dst
}

// rightmostUntrusted is the one hop RightmostUntrusted evaluates: walking from the right, the first hop
// not in trustedProxies. When every hop is trusted the request started inside the proxy chain (a load
// balancer health check, an internal call), so its origin, the leftmost hop, is evaluated.
// Empty only when the chain is empty. The result aliases hops.
func (p Plugin) rightmostUntrusted(hops []string) []string {
	for i := len(hops) - 1; i >= 0; i-- {
		if !p.trustedHop(hops[i]) {
			return hops[i : i+1]
		}
	}
	if len(hops) == 0 {
		return nil
	}
	return hops[:1]
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

// newTrustedProxyHelper stores trustedProxies (CIDRs or bare addresses) on a family-isolated helper.
// nil when no entry is set, so membership costs nothing.
func newTrustedProxyHelper(entries []string) (*iplookup.Helper, error) {
	helper := iplookup.New()
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		if err := helper.AddEntry(entry, ""); err != nil {
			return nil, fmt.Errorf("trustedProxies: invalid entry %q: %w", entry, err)
		}
	}
	if helper.Count() == 0 {
		return nil, nil
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
