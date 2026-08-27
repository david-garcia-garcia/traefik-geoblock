# oschwald maxminddb-golang — Yaegi overlay

Upstream is `github.com/oschwald/maxminddb-golang` (see `go.mod`). We do **not** fork the decoder.

Yaegi loads every `.go` file in an imported package. Upstream `mmap_unix.go` imports `golang.org/x/sys/unix`; Traefik then panics (`incomplete type ifreq`).

After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1`. That:

1. Copies `reader_mmap.go.overlay` over vendor `reader_mmap.go` (`ReadFile` + `FromBytes`). The `.overlay` suffix keeps `go test ./...` from compiling this folder as a package.
2. Deletes `mmap_unix.go` and `mmap_windows.go` so Yaegi never imports `x/sys`.

`x/sys` may still appear in `go.mod` (upstream require). It is unused if nothing imports it. Do not import it.

Upgrade: bump the require, `go mod vendor`, re-run the script. If `Open` / `Close` in `reader_mmap.go` change shape, update the overlay here.
