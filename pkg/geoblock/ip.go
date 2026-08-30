package geoblock

import (
	"net"
	"net/http"
	"strings"
)

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

// canonicalIPHeaders canonicalizes configured IP header names. remoteAddress stays literal.
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
