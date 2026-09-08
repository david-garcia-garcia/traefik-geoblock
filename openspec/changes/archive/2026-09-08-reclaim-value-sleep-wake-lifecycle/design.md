## Context

`pkg/reclaim` on `origin/master` is 260 lines with a seven-field `slot`. Each slot carries a
`context.WithCancel` lifetime, its `cancel`, and a goroutine parked on that context whose only
job is to call `Close()` on the value. Nothing outside the package can observe any of it, because
`life` is never handed to `create` — `create` takes no arguments so Yaegi can call it. Grace is a
`time.AfterFunc` whose queued callback has to be invalidated by a `graceGen` counter, and `Open`
runs `create` outside the table mutex, so two first `Open` calls for one key both create and one
result is thrown away.

Hard constraints this design does not get to move:

- The package stays **stdlib-only, non-generic, storing `any`**. It is a file copy shared with
  `david-garcia-garcia/traefik-modsecurity`. `TestTable_StdlibImports` pins the import rule.
- **Yaegi** runs this interpreted: no generics on cross-package types, and `create` stays
  `func() (any, error)`.
- **Five stable message constants**, all at debug, and **no log line may be emitted while the
  table mutex is held** — a caller-supplied `slog` handler can block or re-enter.
- A holder whose `Done()` is nil (the Yaegi context shape) must keep working.

## Goals / Non-Goals

**Goals:**

- Four events on the stored value, with `sleep` always preceding `close` on every ending path.
- `Open` never returns a sleeping value.
- Ordering of `reclaim_orphan` before `reclaim_dispose` as a structural property, not a flag.
- `reclaim_dispose` emitted only after `Close()` has returned.
- One `create` per key, ever, with no discarded value.
- Remove the per-slot lifetime context, its `cancel`, and the goroutine parked on it.
- `pkg/dbsource.Updater` stoppable deterministically and restartable; BIN and MMDB sleep/wake.

**Non-Goals:**

- A per-incarnation `grace` argument on `Open` (that is PR #82's design; not asked for here).
- Any `wake` failure path, error return, or create-fallback.
- Releasing the BIN/MMDB database handle on sleep (see Decision 6).
- Porting the change to `traefik-modsecurity` (separate act).
- Reworking `BIN.hotSwap` or its ten-second delayed `oldDB.Close()` goroutine.

## Decisions

### 1. Table mutex, not a per-key owner goroutine

Measured in a scratch module outside the repo, three shapes compared:

```
probe 1 — eight racing first Opens, create calls:
  owner-naive 1, owner-reserved 1, mutex-register 1

probe 2 — Open racing an owner that is deregistering, 1600 attempts:
  owner-naive    203 stranded
  owner-reserved   0 stranded
```

The owner goroutine is not what removes the lost-create race — registering the key before
`create` is, and that works with or without a goroutine. What the owner goroutine *adds* is a new
race: an owner that deregisters itself under the table mutex and exits leaves an `Open` that
already took the handle sending into a dead mailbox. Closing that needs a `pending` reservation
counter incremented under the same table mutex before `Open` releases it, plus an owner that
refuses to exit while it is non-zero — a second mutex-guarded invariant on top of the mailbox,
the reply channel, and the idle timer. It also turns the hot path (bind to a live value) from one
mutex acquire into a channel round trip plus a goroutine hop.

Decision: keep the table mutex for `items` and slot state. Every blocking value call (`create`,
`Wake`, `Sleep`, `Close`) runs outside it.

### 2. Four slot states and one `ready` handoff channel

```
busy   — create / Wake / Sleep in flight; Open and drop park on s.ready
awake  — usable; Open binds and returns the value
asleep — slept, kept for grace; Open wakes it, the grace wait disposes it
failed — create returned an error; the key is already unmapped
```

`Open` and `drop` both take the mutex, read the state, and either act or park on `s.ready` and
re-loop. Every state change happens under the mutex; every blocking call happens outside it, with
the slot parked in `busy` so nothing else can act on it meanwhile. That is what makes the state
machine sequential per key without a goroutine per key.

The slot loses `cancel`, `graceTimer`, `graceGen`, and the `holders map[uint64]struct{}` +
`nextID` pair. Holder ids buy nothing once `drop` carries the `*slot` and verifies
`t.items[key] == s`: a watcher fires exactly once and always for its own slot, so a plain count
is enough.

### 3. Register the key before `create`

`Open` publishes a `busy` slot under the mutex, releases it, then calls `create`. A second first
`Open` finds that slot and parks. On success the creator stores the value, binds itself, and
closes `ready`. On failure it records the error, unmaps the key, and closes `ready`, so parked
callers get that error rather than serially retrying the same failing download.

This deletes the lost-create branch, the duplicate GeoIP download, and
`TestTable_LostCreateRaceCancelsLoser`.

### 4. The ending goroutine owns orphan through dispose

`Open` already starts one `watch` goroutine per call that blocks on the holder context and then
calls `drop`. This design gives that goroutine the rest of the incarnation:

```
Sleep(value) → log reclaim_orphan → wait out grace → Close(value) → log reclaim_dispose
```

Sequential statements in one goroutine cannot reorder, so `reclaim_orphan` always precedes
`reclaim_dispose` with no flag, no generation counter, and no window. The grace wait is a
`select` on a timer and the slot's `woken` channel, which a reclaiming `Open` closes — so a
reclaim releases the goroutine immediately rather than leaving it parked for the rest of grace.

No goroutine is added: the watcher was going to exit here anyway. Two goroutines per incarnation
become one, and the `time.AfterFunc` disappears.

### 5. Zero grace never publishes a sleeping value

At `grace == 0` the ending goroutine deletes the key in the same critical section in which it
leaves `busy`, so no `Open` can observe the `asleep` state. A racing `Open` finds the key absent
and creates a new incarnation, logging `reclaim_put` + `reclaim_bind`. That is the approved
behaviour delta; master's `reclaim_reclaim` there was an artifact of the timer check.

### 6. Wrapper sleep releases the update loop, not the database handle

`BIN.Sleep` / `MMDB.Sleep` stop and join the updater; `Wake` restarts it; `Close` releases only
the database handle, because sleep always ran first. That is the "cleanup is never duplicated"
split.

The handle deliberately stays open across a sleep. `Wake` cannot fail in v1, and reopening a file
that may have been rotated or deleted is a failure `Wake` has no way to report — the lifecycle
names "the update ticker, in-flight downloads" as what sleep releases. Releasing the handle needs
a defined `Wake` failure policy first; recorded as follow-up debt, not taken.

### 7. `Updater` gets a joinable, restartable stop

`Start` refuses to double-start and registers a `sync.WaitGroup`; the loop goroutine defers
`Done`. `tick` checks the stop signal before the age check and again before the update callback.
`Stop` stops the ticker, closes the signal once, waits, then clears the ticker and signal so a
later `Start` is clean. Both `Start` and `Stop` take a mutex, because `Sleep`/`Wake` now call them
from a different goroutine than `create` did.

### 8. `BIN` gains a mutex

`MMDB` already has a `sync.RWMutex` over `db`/`path`; `BIN` has none, and `hotSwap` already
mutates `w.db` unguarded. Sleep and wake add a second writer, so `BIN` gets one `sync.Mutex` over
`db`/`path`/`version`/`updater` — the fields sleep, wake, and hot-swap all touch. Symmetry with
`MMDB`, scoped to those fields; not a rewrite of `hotSwap`.

## Risks / Trade-offs

- **The watcher goroutine now lives through grace.** It was exiting at `drop` and a timer took
  over. Net goroutine count still falls (the lifetime goroutine is gone), and a reclaim releases
  the waiter at once via `woken`, but a test that samples goroutines mid-grace sees a different
  shape than master.
- **First real divergence from the shared `traefik-modsecurity` copy.** The two copies are
  currently the same code modulo local variable names. This has to be ported deliberately; it is
  no longer a mechanical file copy plus nothing.
- **`Reset` racing `Open` on the same key is now a documented precondition** rather than a
  handled case: a creator that finds itself unmapped re-registers, exactly as master does. `Reset`
  is tests-only and must not race `Open` on the same key.
- **`Sleep` joins an in-flight download.** A wrapper whose updater is mid-GET blocks the ending
  goroutine for that key until the request finishes. That is the point (deterministic teardown),
  and only that key's transitions are blocked because sleep runs outside the table mutex — but a
  slow source makes an orphan slow.
- **Zero-grace label change is observable.** Anything grepping for `reclaim_reclaim` on a
  zero-grace table sees fewer lines. Zero grace has no grace window by definition, so the old
  label was wrong; stated in the spec so it is not read as a regression.
