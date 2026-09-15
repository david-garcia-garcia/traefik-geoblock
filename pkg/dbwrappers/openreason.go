package dbwrappers

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"

// openReason is why a wrapper published the handle it logs on `BIN opened` / `MMDB opened`.
// One line per published handle: construction and keep-current share the message and differ here.
type openReason string

const (
	// reasonInit is the handle wrapper construction opened from the resolved file.
	reasonInit openReason = "init"
	// reasonSeed is the bundled seed construction opened while a dated promote is still pending.
	reasonSeed openReason = "seed"
)

// updaterReason names a keep-current trigger as an open reason: promote or download.
func updaterReason(trigger dbsource.UpdateTrigger) openReason {
	return openReason(trigger)
}
