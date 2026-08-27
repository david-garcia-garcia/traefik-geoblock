//go:build !appengine && !plan9 && !js && !wasip1 && !wasi
// +build !appengine,!plan9,!js,!wasip1,!wasi

package maxminddb

import "os"

// Open loads a MaxMind DB file into memory and returns a Reader.
// Overlay: do not mmap. Upstream mmap pulls golang.org/x/sys, which
// Yaegi cannot interpret (incomplete type ifreq on Linux).
func Open(file string) (*Reader, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return FromBytes(buf)
}

// Close returns the resources used by the database to the system.
func (r *Reader) Close() error {
	r.buffer = nil
	return nil
}
