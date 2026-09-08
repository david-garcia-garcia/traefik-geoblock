# Yaegi drops the method set of a value returned as `any`

Status: open, upstream
Found: 2026-09-08, while chasing an Integration Tests failure on `2026-09-08-reclaim-lifecycle`
Scope: `pkg/reclaim` optional interfaces (`closer`, `sleeper`, `waker`) when the plugin runs
interpreted inside Traefik. Not reachable from `go test`, which compiles.

## What happens

`Table.Open` takes `create func() (any, error)` and stores what it returns. Under Yaegi
(v0.16.1, the version Traefik v3.7.11 pins), a value returned through an interpreted function
whose result type is `any` comes back as a synthesized struct type with its methods stripped:

    fmt.Sprintf("%T", value)          // *struct { Xn int }, not *dbwrappers.BIN
    reflect.TypeOf(value).String()    // same
    value.(sleeper)                   // never matches
    switch value.(type) { case sleeper: }  // never matches

So none of the optional lifecycle methods run interpreted. `Sleep`, `Wake`, and `Close` are all
inert in production; a wrapper's update ticker keeps running while the value is unheld, which is
the exact cost this change set out to remove.

This is not new. `closer` has the same shape and has never fired under Yaegi, so no stored
database handle has ever been closed in production either. Verified against `origin/master`.

## What was measured

A probe module outside the repo runs the real packages out of a `plugins-local` GoPath through
`interp` + `stdlib` + `unsafe`, the way Traefik's local-plugin loader does. Results:

| shape | result |
| --- | --- |
| concrete value passed into a `func(any)` parameter, type switch | matches |
| same, comma-ok assertion | panics (`reflect.Set: ... interp.valueInterface`) |
| value returned through an interpreted `func() (any, error)`, type switch | no match |
| same, comma-ok assertion | no match |
| `reflect.ValueOf(v).MethodByName("Sleep")` | not found |
| `reflect.TypeOf(v).Implements(...)` | panics |
| concrete assertion `v.(*target)` on the returned value | matches |

Only the concrete assertion survives, and a shared component that stores `any` cannot name a
concrete type.

## What was done now

The three lookups became single-case type switches. That does not make the methods run; it
removes the comma-ok panic shape, which would land on a background goroutine and take the whole
Traefik process down rather than failing one middleware.

## What would fix it

Reaching the value's methods interpreted needs the caller to hand the lifecycle over explicitly
rather than have `Table` discover it, for instance a create that returns the value together with
its optional hooks. That changes `Open`'s signature, which is fixed by two constraints: Yaegi
cannot call `func(context.Context) (any, error)`, and `pkg/reclaim` is a byte-for-byte copy
shared with `david-garcia-garcia/traefik-modsecurity`. Both repos would have to move together.

Worth re-testing on a Yaegi bump: if upstream preserves method sets across an `any` return, the
current code starts working with no change.

## No CI guard for interpreted-only breakage

`go test` compiles, so it cannot see any of this. The same run turned up a second Yaegi-only
defect in `pkg/dbsource`: `parsed.RawQuery, parsed.Fragment, parsed.User = "", "", nil` passes
every compiled check and panics interpreted, on the update goroutine, killing Traefik. Only the
Pester Integration Tests caught it, and only as "Traefik API accessible" failing, which points
nowhere near the cause.

A cheap guard is a CI step that builds a throwaway module outside this one, depends on
`github.com/traefik/yaegi` at the version Traefik pins, and evaluates the plugin out of a
`plugins-local` GoPath. Keeping it out of this module matters: `go.mod` and `vendor/` here are
what Traefik interprets, and yaegi does not belong in them.
