# Integration Tests failure: what it was

## Symptom

All 10 Pester tests failed on `ff89fef5` and `cbb079fa`, including the baseline "Should have
Traefik API accessible". Traefik never served traffic, so the failure was the process, not the
plugin's decisions. `go test` and golangci-lint were green on the same tree, which put the cause
in the interpreter rather than in the compiled behaviour.

## How it was reproduced

Docker on this box is in Windows-containers mode and the compose images have no manifest for it,
so the stack could not be brought up locally. Instead a probe module outside the repo
(`D:/repositories/scratch-yaegi-geoblock`) does what Traefik's local-plugin loader does:
`interp.New` with `GoPath` pointing at a `plugins-local` tree, `Use(stdlib.Symbols)` and
`Use(unsafe.Symbols)` (compose sets `useunsafe=true`), then evaluate the package. Yaegi v0.16.1,
the version Traefik v3.7.11 pins.

The plugin's `New` resolved fine, so it was not a load failure. Driving the real `pkg/reclaim`
and `pkg/dbsource` through the interpreter found it.

## Cause

`withoutQuery` in `pkg/dbsource/updater.go`, added by the security axis to keep the download
token out of error logs, ended with:

    parsed.RawQuery, parsed.Fragment, parsed.User = "", "", nil

A multi-assignment that mixes an untyped `nil` into a pointer field panics under Yaegi with
`reflect: New(nil)`. The same three assignments written one per line are fine. Confirmed both
ways in the probe.

That line runs only when a download fails. In the compose stack the ipinfo and maxmind sources
carry empty tokens in CI, so their first update attempt fails immediately, the update goroutine
panics, and an unrecovered panic on a goroutine ends the Traefik process. `restart:
unless-stopped` then loops it, and every request in the suite fails, starting with the API check.

Fixed by splitting the assignment, with a comment saying why it is split.

## Second finding, pre-existing, not the cause

While confirming the lifecycle interpreted, the optional interfaces turned out never to match
under Yaegi: a value returned through an interpreted `func() (any, error)` comes back as a
synthesized struct type with no methods. `Sleep`, `Wake`, and `Close` are all inert interpreted.
`closer` has the same shape on `origin/master`, so this predates the branch and is not a
regression, but it does mean the ticket's saving is real only for compiled callers today.

The lookups moved to single-case type switches, which does not make them match but removes the
comma-ok form, which *panics* under Yaegi for a value that reached `any` by being passed in.
Given the same panic-on-a-goroutine consequence, that shape should not be in this package.

Written up with the full measurement matrix in
`knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md`.
