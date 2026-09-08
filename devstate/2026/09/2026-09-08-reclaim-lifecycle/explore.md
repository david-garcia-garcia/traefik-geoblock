# Explore

Baseline is `origin/master` (`d10ad52`): `pkg/reclaim/table.go` 260 lines, `slot` 7 fields,
`table_test.go` 26 tests. Prior art from PR #82 (`2026-09-08-fix-reclaim`) is surveyed in
`priorart.md` next to this file. That branch is read, not merged.

## Concepts

**Incarnation.** One instance of a value under a key. Begins at the `Open` that found the key
absent and ran `create`; ends when the value is closed. A reclaim keeps the same incarnation
(same pointer). Today an incarnation has two events; this change gives it four.

**Sleep / wake.** `sleep` runs when the last holder's context is Done; the value stays stored
with the same identity and releases what is expensive to hold idle. `wake` runs before `Open`
hands a stored sleeping value back. They are a matched, repeating pair.

**Grace, redefined.** Today: how long the table waits before it cancels the lifetime. After this
change: **how long a sleeping value is kept before it is disposed.** The reason to keep grace
long stops being "reloads are fast" and becomes "a sleeping value is cheap to keep".

**Ending goroutine.** The `watch` goroutine that observes a holder context become Done already
runs `drop`. This change makes that goroutine own the whole rest of the incarnation: sleep, the
`reclaim_orphan` line, the grace wait, `Close()`, and the `reclaim_dispose` line. One goroutine,
its lines in order, by construction.

## Decisions

### D1 — No per-key owner goroutine. Table mutex plus an explicit slot state.

Measured in a scratch module at `%TEMP%\reclaim-owner-probe` (outside the repo), three shapes
compared, `go run .`:

```
== probe 1: two first Opens for one key, does create run twice? ==
  owner-naive    creates=1 (want 1)
  owner-reserved creates=1 (want 1)
  mutex-register creates=1 (want 1)

== probe 2: Open racing an owner that is deregistering ==
  owner-naive    stranded Opens: 203 (each one blocked 200ms then failed)
  owner-reserved stranded Opens: 0
```

Two facts, both requested by the ticket:

1. **The owner goroutine is not what removes the lost-create race.** All three shapes ran
   `create` exactly once for eight racing first `Open`s. What removes it is *registering the key
   before `create` runs*, and that works identically with or without a goroutine.
2. **The owner goroutine moves the race rather than removing it.** In `owner-naive` the owner
   deregisters itself under the table mutex and returns; an `Open` that took the handle under the
   mutex and sent after releasing it hit a departed owner **203 times out of 1600**. Closing that
   window needs a `pending` reservation counter incremented under the table mutex before `Open`
   releases it, plus an owner that refuses to exit while `pending > 0` (`owner-reserved`, 0
   stranded). That is a second table-mutex-guarded invariant *on top of* the mailbox, the reply
   channel, the idle timer, and the goroutine — strictly more machinery than the mutex shape, for
   the same guarantee.

Cost side: with an owner goroutine every `Open` — including the hot path of binding to a live
value — becomes a channel round trip and a goroutine hop instead of one mutex acquire, and every
key holds a goroutine for as long as it is mapped.

Decision: keep the table mutex. It guards `items` and slot state only; all blocking value work
(`create`, `Wake`, `Sleep`, `Close`) runs outside it, serialized by an explicit slot state plus a
`ready` handoff channel.

### D2 — Register the key before `create` runs.

`Open` publishes a slot in state `busy` under `t.mu`, releases the mutex, then runs `create`. A
second first `Open` finds the slot, parks on `s.ready`, and re-loops when the transition ends.
The lost-create branch, the duplicate GeoIP download, and `TestTable_LostCreateRaceCancelsLoser`
all go away. A failed `create` stores the error on the slot, unmaps the key, and closes `ready`,
so parked callers get that error instead of serially retrying the same failing download.

### D3 — Four slot states, one `ready` channel, no ordering flag.

```
busy   — create / Wake / Sleep in flight; Open and drop park on s.ready
awake  — value is usable; Open binds and returns it
asleep — slept, kept for grace; Open wakes it, the grace wait disposes it
failed — create returned an error; the key is already unmapped
```

`slot` becomes 7 fields with no `context.CancelFunc`, no `*time.Timer`, no generation counter and
no holder map:

```go
type slot struct {
	value   any
	err     error
	state   slotState
	ready   chan struct{}
	woken   chan struct{}
	holders int
	logger  *slog.Logger
}
```

`holders` can be a plain count rather than master's `map[uint64]struct{}` + `nextID`, because
`drop` already carries the `*slot` and verifies `t.items[key] == s`. A watcher fires exactly once
and always for its own slot, so ids buy nothing. That removes a per-slot map allocation.

`graceGen` disappears because there is no queued `AfterFunc` to invalidate: the ending goroutine
holds the grace wait itself and `woken` cancels it.

### D4 — The ending goroutine owns orphan → dispose, so ordering is structural.

`arming` on PR #82 exists to keep `reclaim_orphan` ahead of `reclaim_dispose` while neither line
may be written under `t.mu`. It caused the stranded-incarnation defect. Master has the same
hazard in the other direction (it arms the timer *before* logging orphan, so a sub-millisecond
grace can log `dispose` first — reproduced there as `[put, bind, dispose, orphan]`).

Here, one goroutine runs, in order: `Sleep(value)` → log `reclaim_orphan` → wait out grace →
`Close(value)` → log `reclaim_dispose`. Sequential statements in one goroutine cannot reorder.
No flag, no generation, no window.

Both guarantees the ticket says must survive fall out of that: dispose is logged after `Close()`
returned, and orphan always precedes dispose for one incarnation at every grace. Note that
`origin/master` does **not** provide either today, so this change adds them.

### D5 — Optional interfaces beside `closer`; `create` unchanged.

```go
type closer  interface{ Close() }
type sleeper interface{ Sleep() }
type waker   interface{ Wake() }
```

Same comma-ok assertion shape as today's `closer`, so a value that implements none is unaffected
and Yaegi sees no new cross-package surface. `create` stays `func() (any, error)`. `Wake` has no
error return (v1 decision).

### D6 — Zero grace never publishes a sleeping value.

At `grace == 0` the ending goroutine deletes the key in the same critical section that leaves
`busy`, so no `Open` can ever observe the `asleep` state. A racing `Open` finds the key absent
and creates a new incarnation: it logs `reclaim_put` + `reclaim_bind`, which is the approved
behaviour delta ("a plain bind, not a reclaim"). Master's `reclaim` label there was an artifact.

### D7 — `Reset` emits orphan before dispose too.

Master's `Reset` logs only `reclaim_dispose`. Since every ending path must now sleep first, a
`Reset` on a live incarnation calls `Sleep`, logs orphan, calls `Close`, logs dispose. A slot
already `asleep` has had its orphan line; `Reset` does not repeat it. That makes
"orphan precedes dispose" exceptionless rather than a rule with a `Reset` carve-out.

`Reset` also closes each slot's `woken` channel so grace waits stop promptly, and a slot that is
`busy` at `Reset` time is disposed by the goroutine that owns that transition — a uniform rule:
**a transition that finishes and finds its slot unmapped closes the value itself.**

### D8 — `Updater` gets a joinable, restartable stop.

`Start` refuses to double-start, registers a `sync.WaitGroup`, and the goroutine defers `Done`.
`tick` checks `stop` before `UpdateIfNeeded` and again before `onUpdate`. `Stop` stops the
ticker, closes `stop` once, `Wait()`s, then clears `ticker`/`stop` so a later `Start` is clean.
That is the concrete bug sleep exists for: a disposed wrapper writing into a `TempDir` that a
test is removing.

### D9 — Wrapper sleep releases the updater, not the database handle.

`BIN.Sleep` / `MMDB.Sleep` stop and join the updater. `Wake` restarts it. `Close` closes the file
handle only — no updater handling, because sleep always ran first. That is the "cleanup is never
duplicated between sleep and close" split the ticket asks for.

The database handle deliberately stays open across a sleep. `Wake` cannot fail in v1 (a hard
decision), and reopening a file that may have been rotated or deleted is a failure `Wake` has no
way to report. The lifecycle definition names "the update ticker, in-flight downloads" as what
sleep releases; releasing the handle as well needs a defined `Wake` failure policy first. Noted
as follow-up debt, not taken.

### D10 — `Open` keeps master's four-argument signature.

PR #82 added a per-incarnation `grace` argument. This branch starts from `master` and the ticket
does not ask for it, so `Open(ctx, key, logger, create)` stays as-is (`skill:sbs-dev-commandments:Bound the ask`).

## Open questions

- Q: Does an existing sleep/wake reclaim implementation already live in another repository owned
  by `david-garcia-garcia`, which this change should follow instead of inventing?
  Decision: assumed — none found. PR #82's own card records the same search resolved as "no":
  upstream `traefik-modsecurity` `pkg/reclaim` is three files with zero hits for `Wake`/`Sleep`.
  A repository-wide GitHub search is running in parallel; if it lands with a hit before implement
  closes, follow it. Otherwise design from scratch as above.
  By: explore
- Q: Should `Sleep` also release the BIN/MMDB database handle, not only the updater?
  Decision: assumed — no, not in v1. `Wake` must be infallible and a reopen can fail. Recorded as
  `knowledge/debt/` follow-up so the human can rule on a `Wake` failure policy later.
  By: explore
- Q: PR #82 is open on the same package and will conflict. Should this branch rebase onto it?
  Decision: resolved — no. The human explicitly accepted the rework and the future conflict, and
  told this run to branch from `origin/master` and read #82 only as prior art.
  By: explore
- Q: PR #82's card records a human decision "do not delete the lifetime context and its per-slot
  goroutine" under the shared-copy no-removal rule. This change deletes them. Is that reversed?
  Decision: resolved — yes, reversed for this run. The ticket states the rule protects
  functionality, not unobservable plumbing, and `life` is never handed to `create`, so nothing
  outside the package can observe it. The package stays stdlib-only, non-generic, and `any`, so
  the port to `traefik-modsecurity` is still a file copy.
  By: explore
- Q: `waitCtx` polls `ctx.Err()` every 20 ms for a holder whose `Done()` is nil (the Yaegi shape).
  Does the redesign change that?
  Decision: assumed — no. The watcher goroutine and `waitCtx` stay exactly as on master; only
  what `drop` does afterwards changes. Keeping it means the Yaegi holder case is untouched.
  By: explore
- Q: `BIN` has no mutex and `hotSwap` mutates `w.db` unguarded; `Sleep`/`Wake` add another writer.
  Decision: assumed — add a `sync.Mutex` to `BIN` guarding `db`/`path`/`version`/`updater`, mirroring
  the `sync.RWMutex` `MMDB` already has (`skill:sbs-dev-commandments:Symmetry and consistency`).
  Scoped to the fields sleep/wake and hot-swap both touch; not a rewrite of `hotSwap`.
  By: explore
- Q: The 10-second delayed `oldDB.Close()` goroutine in `BIN.hotSwap` is unowned and unjoinable.
  Does sleep have to cancel it?
  Decision: assumed — no. It holds a stale handle that no caller can reach, and joining it would
  make `Sleep` block for ten seconds. Out of scope per `requirement.md`.
  By: explore
