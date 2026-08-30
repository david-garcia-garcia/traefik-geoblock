package geoblock

import (
	"fmt"
	"net/http"
)

// Route is one Traefik New: the shared Plugin and this next.
type Route struct {
	*Plugin
	next http.Handler
}

// ForRoute returns a Route that serves next. The Plugin already holds catalog lookup.
func (p *Plugin) ForRoute(next http.Handler) (*Route, error) {
	if next == nil {
		name := "geoblock"
		if p != nil {
			name = p.name
		}
		return nil, fmt.Errorf("%s: no next handler provided", name)
	}
	return &Route{Plugin: p, next: next}, nil
}

// ServeHTTP calls Plugin.ServeHTTP with this next.
func (r *Route) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	r.Plugin.ServeHTTP(rw, req, r.next)
}

// SameCore reports whether a and b hold the same stored Plugin.
func (r *Route) SameCore(q *Route) bool {
	if r == nil || q == nil || r.Plugin == nil || q.Plugin == nil {
		return false
	}
	return r.Plugin == q.Plugin
}

// Next is this route’s downstream handler.
func (r *Route) Next() http.Handler {
	if r == nil {
		return nil
	}
	return r.next
}
