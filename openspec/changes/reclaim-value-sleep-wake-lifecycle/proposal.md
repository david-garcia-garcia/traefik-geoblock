## Why

A value stored on the reclaim table has two events, create and close. When its last holder's
context is Done the incarnation stays fully live for the whole grace period, so a GeoIP database
keeps its 24-hour update ticker armed for a resource nobody is using — and `pkg/dbsource.Updater`
cannot be joined, so a wrapper the table already disposed can still download and write into a
directory that is being removed.

Adding the two missing events is only half of it. The table's current shape makes them expensive
to express: an unobservable per-slot `context.WithCancel` lifetime with a goroutine parked on it,
a grace timer whose queued callback has to be invalidated by a generation counter, and a
create-outside-the-lock window that lets two first `Open` calls both download the same database.

## What Changes

- A stored value gets a four-event lifecycle: `create -> (sleep -> wake)* -> sleep -> close`.
  `sleep` releases what is expensive to hold idle when the last holder goes; `wake` runs before
  `Open` hands the value back; `close` is always preceded by `sleep` on every ending path, so
  cleanup is never duplicated between the two.
- `Sleep()` and `Wake()` join `Close()` as **optional** interfaces on the stored value, asserted
  the same way. A value that implements none behaves exactly as today. `create` keeps its
  `func() (any, error)` signature (Yaegi cannot call `func(context.Context) (any, error)`).
- **BREAKING** (behaviour, not API): grace is redefined as *how long a sleeping value is kept
  before it is disposed*, not how long a fully live value is kept.
- `Open` registers the key **before** it runs `create`, so a second first `Open` waits for the
  first result instead of running a second `create` whose value is thrown away. The lost-create
  race and its spec scenario are removed.
- The per-slot lifetime context, its `cancel`, and the goroutine parked on it are removed. The
  table closes the value directly and emits `reclaim_dispose` only after `Close()` has returned.
- `reclaim_orphan` is emitted after `Sleep()` returns and always precedes the `reclaim_dispose`
  of the same incarnation, at every grace, with no exception for `Reset`. The ordering is
  structural: one goroutine runs sleep, orphan, the grace wait, close, and dispose in sequence.
- **BREAKING** (log labels): at zero grace, an `Open` that races the last holder going away is
  logged as `reclaim_put` + `reclaim_bind`, not `reclaim_reclaim`. A zero-grace table keeps no
  sleeping value, so there is no grace window to reclaim into.
- `pkg/dbsource.Updater.Stop` joins its goroutine, `tick` refuses to download after stop, and
  `Start` after `Stop` restarts cleanly.
- `pkg/dbwrappers` BIN and MMDB implement `Sleep()`/`Wake()`: sleep stops and joins the update
  ticker, wake restarts it, and `Close()` is reduced to releasing the database handle.

## Capabilities

### New Capabilities

- `std_go_reclaim_value-lifecycle`: the four events a stored value receives, their order, the
  optional interfaces that carry them, and the guarantee that `close` is always preceded by
  `sleep` on every ending path.

### Modified Capabilities

- `std_go_reclaim_context-lease`: grace is redefined as how long a *sleeping* value is kept;
  `Open` creates once by registering the key before `create` (the lost-create race is gone);
  `reclaim_orphan` and `reclaim_dispose` gain ordering and completion guarantees.
- `core_geoblock_database_wrapper-reclaim`: the BIN and MMDB wrappers implement sleep and wake,
  so an unheld wrapper stops its update ticker instead of staying fully live for the grace.
- `core_geoblock_database_url-download`: the update loop stops deterministically — `Stop` joins
  the goroutine and a stopped updater performs no further download — and can be restarted.

## Impact

- `pkg/reclaim/table.go`, `pkg/reclaim/default.go`, `pkg/reclaim/table_test.go`
- `pkg/dbsource/updater.go` and its tests
- `pkg/dbwrappers/bin.go`, `pkg/dbwrappers/mmdb.go`, `pkg/dbwrappers/reclaim_test.go`
- `knowledge/devdocs/std_go_reclaim.md`
- `pkg/reclaim` is a copy shared with `david-garcia-garcia/traefik-modsecurity`. It stays
  stdlib-only, non-generic, and stores `any`, so the port remains a file copy — but this change
  is the first real behavioural divergence between the two copies and has to be ported.
- No configuration surface changes. No persisted data changes.
