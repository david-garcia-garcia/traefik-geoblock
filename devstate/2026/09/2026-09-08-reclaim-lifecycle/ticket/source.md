# Ticket source (local)

IssueKey: 2026-09-08-reclaim-lifecycle
Host: local (caller spec, no tracker)

## The requirement

`pkg/reclaim` today gives a stored value two events, create and close. An incarnation that has
lost its last holder stays fully alive for the whole grace period, so a GeoIP database keeps its
update ticker running and its file open for a resource nobody is using. Give the value a
four-event lifecycle, and restructure the table so those states are simple to express.

### Lifecycle (decided by the human, do not redesign)

`create -> (sleep -> wake)* -> sleep -> close`

- `create` runs once per incarnation, on the first `Open` for a key that finds it absent. The
  value comes back awake.
- `sleep` runs when the last holder's context is Done. The value stays stored and keeps its
  identity (a later `Open` returns the same pointer), and releases what is expensive to hold
  idle: the update ticker, in-flight downloads.
- `wake` runs when an `Open` arrives for a stored, sleeping value. `Open` MUST NOT return before
  `wake` has returned. A caller never receives a sleeping value.
- `close` runs when the value is dropped for good, and is ALWAYS preceded by `sleep` on every
  ending path. A value's `close` therefore never has to handle the live state, and cleanup code
  is never duplicated between sleep and close. This is the point of the split, per the human:
  "sleep is always called before close, even if there is no grace, this avoids repeating
  cleanups".
- `sleep` and `wake` are a matched, repeating pair. `create` and `close` happen once each.
- Every ending path obeys sleep-before-close: grace expiry (already asleep, must not sleep
  twice), `Reset` on a live incarnation (sleep then close), and the loser of a create race
  (create, sleep, close, even though it was never bound).
- Zero grace means create, sleep, close back to back. There is no window in which a sleeping
  value could be woken, so it is not kept.
- `wake` cannot fail in v1 (decided). A value that cannot guarantee resume simply does not
  implement sleep/wake. Do not add an error return or a create-fallback edge.

### API shape

Optional interfaces, exactly like today's `closer`, so values that do not implement them are
unaffected and Yaegi sees nothing new. Do NOT change `create`'s signature: it must stay
`func() (any, error)`, because Yaegi cannot call `func(context.Context) (any, error)`.

Grace gets an honest definition as a result: how long a sleeping value is kept before it is
disposed. Say that in the spec and in the devdocs, and note that the reason to keep grace long
stops being "reloads are fast" and becomes "a sleeping value is cheap to keep".

### Second thread, same ticket: structural simplification

The current table is about 320 lines with a nine-field slot, and a design review concluded
roughly a third of it is accidental complexity. Address these:

- Each slot carries a `context.WithCancel` lifetime, a `cancel`, a goroutine parked on it, and a
  `valueClosed` channel so dispose can wait for that goroutine. Nothing outside the package can
  observe any of it, because `life` is never handed to `create`. The human has confirmed that the
  standing "never remove functionality from this shared component" rule protects functionality,
  not unobservable plumbing, so this indirection should go: close the value directly and log
  dispose after it returns.
- `slot.arming` exists only because log lines must not be emitted under the table mutex while
  `reclaim_orphan` must still precede `reclaim_dispose`. It produced a real stranded-incarnation
  bug (a slot left mapped with no holders, no timer, and a value never closed). Replace it with
  an ordering property: a per-slot logging mutex always acquired under `t.mu` and released after
  it, or a design where one goroutine owns the key and emits all its lines in order.
- Two first `Open`s for one key both run `create` and one result is thrown away, which for a
  GeoIP database is a duplicated download and file open. Registering the key before running
  create, so the second caller waits for the first's result, removes both the waste and the
  entire lost-create branch.
- Strongly consider a per-key owner goroutine (one goroutine owning the slot, receiving
  bind/drop/expire messages, no mutex on slot state). With sleep/wake the state machine grows to
  LIVE, SLEEPING, ASLEEP, WAKING, and layering that onto mutex-plus-flags is where it gets ugly,
  whereas a sequential select loop makes it trivial. This is a recommendation, not a decision:
  settle it in explore with evidence, and specifically demonstrate whether it keeps the
  lost-create race gone rather than merely moving it, since an owner goroutine must deregister
  itself before exiting and `Open` must handle grabbing a handle to a goroutine that is already
  leaving.

### Behavior deltas already approved by the human

- At zero grace, an `Open` that races the orphan log is logged as a plain bind, not a reclaim.
  Today's "reclaim" label there is an artifact of the arming flag, and a zero-grace table has no
  grace window by definition.

### First consumer

`pkg/dbsource.Updater`. Its `Stop` signals its goroutine but does not join it, and `tick` never
checks for stop before downloading, so a disposed wrapper can still write into a directory a
test's `TempDir` is removing. That bug is the concrete reason sleep exists: sleep should stop the
ticker and join it. Fixing `Updater` so it can be stopped and restarted deterministically is in
scope, and so is wiring the BIN and MMDB wrappers in `pkg/dbwrappers` to implement sleep/wake.

## Non-negotiables

- `pkg/reclaim` is a copy shared with `david-garcia-garcia/traefik-modsecurity` and kept in sync
  in both directions. It must stay stdlib-only, non-generic, and store `any`, so the port stays a
  file copy. `TestTable_StdlibImports` pins the import rule.
- Yaegi runs this code interpreted inside Traefik: no generics on cross-package types, and
  `create` takes no arguments.
- The logging contract: five stable message constants, all at debug, and log lines must never be
  emitted while holding the table mutex, because a caller-supplied slog handler can block or
  re-enter.
- Every log line should mean the work it names has already happened: dispose after close returns,
  orphan after sleep returns.

## Done when

- A PR is open against `master` and all three CI jobs (Test, Lint, Integration Tests) are green
  on its final head.
- The seven-axis code review has run on Opus and every finding is applied or argued with a
  measurement.
- The OpenSpec change is archived and both spec-librarian validators exit 0.
- The delivery card is upserted on the PR summary.
