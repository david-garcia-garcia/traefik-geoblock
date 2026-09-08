# Release the BIN and MMDB database handle on sleep

IssueKey: 2026-09-08-reclaim-lifecycle
Size: large
Action: note

## Why this follow-up

A sleeping wrapper still holds its database open: `*ip2loc.DB` plus its process-temp copy for
BIN, and the whole file read into memory as a `*maxminddb.Reader` for MMDB. For a large MaxMind
database that is tens of megabytes held for a resource nobody is using — the same waste the
update ticker had before this change, in the one place sleep does not yet reach.

## Why it was not taken

`Wake` cannot fail in v1. That is a decision on the ticket, not a preference: the table offers no
error return for wake and never falls back to `create`. Releasing the handle makes wake fallible,
because the file may have been rotated, replaced by a hot-swap, or deleted between sleep and
wake, and a failed reopen would hand the caller a wrapper whose lookups all error. Choosing what
happens then — keep the old handle, fail the `Open`, fall back to `create`, or serve degraded —
is a contract decision the human has to make first.

## Risks

Until it is taken, a long grace on a large MMDB source trades memory for reload speed, and the
devdocs claim that "a sleeping value is cheap to keep" is only true of the ticker, not of the
bytes. A reader may size grace on that claim.

## Context

Current: `pkg/dbwrappers/bin.go` `BIN.Sleep`/`BIN.Close`, `pkg/dbwrappers/mmdb.go`
`MMDB.Sleep`/`MMDB.Close` — sleep stops and joins the updater only.
Proposed: sleep also releases `db` (and for BIN the temp copy at `currentLocalDbCopy`); wake
reopens from `path`, under a defined failure policy for a reopen that fails.
