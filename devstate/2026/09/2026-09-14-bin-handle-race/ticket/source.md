# Fix F-1 and F-1b BIN handle race

Fix F-1 and F-1b from d:\repositories\traefik-geoblock\geoblock-bug-hunt-2026-09-14.md.

F-1: pkg/dbwrappers BIN.db is written by hotSwap and close while LookupRecord reads it, with no mutex. MMDB already has sync.RWMutex for the same job. Race detector reports it; existing TestNew_ContextBindsWrapper and TestOpenBIN_HashChangeDisposesOld fail under go test -race. Production: undefined country data on allow/block for every IP2Location BIN middleware; updater tick and reclaim Close race request goroutines.

F-1b: LookupRecord reads w.db twice (nil check then Get_all). close() nils between those reads → Get_all on nil *ip2loc.DB panics (d.metaok). Same mutex + read-once local as MMDB.Lookup.

In scope: BIN mutex discipline matching MMDB; LookupRecord takes the handle once; tests that fail today under -race must pass; do not change MMDB unless needed for symmetry of the same helper. Optional smallest CI note: if adding -race to CI is a large separate job (needs gcc on runners), note it on issues.md — do not expand this ticket into a CI overhaul unless it is a one-line flag that already works.

Out of scope: F-2 through F-9, bypassHeaders, CIDR trees, updater Stop join (F-4), proof-file filenames zzz_proof_*.

Proof tests in the caller workspace are untracked and must NOT be copied as zzz_proof_*. Write proper product tests. Re-measure: race detector needs cgo; this Windows host may lack gcc — use docker golang:1.25 with -e GOFLAGS=-mod=vendor if go test -race fails locally, same as the report.
