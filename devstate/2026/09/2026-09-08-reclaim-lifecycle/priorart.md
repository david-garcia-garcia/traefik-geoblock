# Prior art for the `pkg/reclaim` lifecycle redesign

Read-only survey. Two trees are compared:

- **fix-reclaim** = branch `2026-09-08-fix-reclaim`, head `9badfbe`, reviewed head `ec6db3e`, GitHub PR [#82](https://github.com/david-garcia-garcia/traefik-geoblock/pull/82). Verified byte-identical to the worktree at `D:/repositories/wt-modsec-2026-09-08-fix-reclaim`.
- **master** = `origin/master` (the baseline the redesign starts from).

| File | master | fix-reclaim |
| --- | --- | --- |
| `pkg/reclaim/table.go` | **260 lines** | **322 lines** |
| `pkg/reclaim/default.go` | 44 lines | 48 lines |
| `pkg/reclaim/table_test.go` | 896 lines / **26 `Test*` funcs** | 1748 lines / **47 `Test*` funcs** |
| `pkg/dbwrappers/reclaim_test.go` | 327 lines / 4 `Test*` funcs | 406 lines |
| `knowledge/devdocs/std_go_reclaim.md` | 57 lines | 67 lines |
| `openspec/specs/std_go_reclaim_context-lease/spec.md` | 109 lines | 162 lines |

`pkg/reclaim` is exactly three files on both branches: `table.go`, `default.go`, `table_test.go`. No `sleep.go`, no `Sleep`, no `Wake` anywhere in either tree (the fix-reclaim card records that the same search on upstream `traefik-modsecurity` also returns zero hits).

**Correction to the brief:** there is no "returned release handle" in either version. `Open` returns `(any, error)` only. Release is implicit: `Open` spawns a `watch` goroutine per call that blocks on the holder context and then calls `drop`. Any redesign that removes per-slot goroutines has to replace that mechanism, not a handle type.

---

# 1. `pkg/reclaim/table.go` on fix-reclaim (322 lines)

## 1.1 Package-level constants

```go
const (
	DefaultGrace = 10 * time.Second

	// TableGrace is the grace an Open passes to take the table's grace instead of naming its own.
	// Any negative duration means the same. It is an Open argument only: NewTable accepts it, but
	// there a negative grace means DefaultGrace, since a table has no table above it to inherit from.
	TableGrace time.Duration = -1
)

const (
	MsgPut     = "reclaim_put"
	MsgBind    = "reclaim_bind"
	MsgOrphan  = "reclaim_orphan"
	MsgReclaim = "reclaim_reclaim"
	MsgDispose = "reclaim_dispose"
)
```

`DefaultGrace` is untyped-ish (`10 * time.Second`, a `time.Duration`). `TableGrace` is explicitly typed `time.Duration = -1`. The round-2 review flagged that `NewTable(TableGrace)` compiles and silently means 10s; that hazard is closed by doc comment, not by the type system.

## 1.2 The five log messages, their exact values, and their structured fields

Every one of the five is emitted at **`slog.Level Debug`** via `logger.Debug(...)`, and every one carries **exactly one** structured field: the literal attribute key `"key"` whose value is the table key string. There are no other attribute keys anywhere in the package.

| Constant | Exact string value | Emitted at | Logger used | Fields |
| --- | --- | --- | --- | --- |
| `MsgPut` | `"reclaim_put"` | `Open`, after first store, outside `t.mu` | that `Open`'s `logger` argument | `"key", key` |
| `MsgBind` | `"reclaim_bind"` | `logBind`, on every successful `Open` | that `Open`'s `logger` argument | `"key", key` |
| `MsgOrphan` | `"reclaim_orphan"` | `drop`, after claiming the arming window, mutex released | `e.logger` (last binding `Open`) | `"key", key` |
| `MsgReclaim` | `"reclaim_reclaim"` | `logBind`, only when `bindLocked` reported `reclaimed` | that `Open`'s `logger` argument | `"key", key` |
| `MsgDispose` | `"reclaim_dispose"` | `fire` (after `<-e.valueClosed`) and `Reset` (after `<-e.valueClosed`) | `e.logger` (last binding `Open`) | `"key", key` |

Call sites verbatim:

```go
logger.Debug(MsgPut, "key", key)          // Open
logger.Debug(MsgReclaim, "key", key)      // logBind
logger.Debug(MsgBind, "key", key)         // logBind
orphanLog.Debug(MsgOrphan, "key", key)    // drop
disposeLog.Debug(MsgDispose, "key", key)  // fire
e.logger.Debug(MsgDispose, "key", key)    // Reset
```

## 1.3 `Table` struct — exact fields

```go
type Table struct {
	mu    sync.Mutex
	grace time.Duration
	items map[string]*slot
}
```

| Field | Type | Purpose |
| --- | --- | --- |
| `mu` | `sync.Mutex` | Guards `items` and every mutable slot field. Log lines must never be emitted while held (spec requirement). |
| `grace` | `time.Duration` | The table default grace, handed to any `Open` that passes `TableGrace`. Fixed at `NewTable`. |
| `items` | `map[string]*slot` | Key → current incarnation. Absence of a key means no incarnation. |

## 1.4 `slot` struct — exact fields (fix-reclaim)

```go
// slot is one incarnation: the value, the cancel for its lifetime, and the holders that still need it.
type slot struct {
	value   any
	cancel  context.CancelFunc
	holders map[uint64]struct{}
	nextID  uint64
	// grace is this incarnation's wait after its last holder goes, fixed by the Open that created
	// it. The table's grace is what an Open takes when it names none. A later Open never changes it.
	grace time.Duration
	// valueClosed is closed by the lifetime goroutine once it has run Close on the value.
	// fire and Reset receive on it before they log dispose, so every slot must have one.
	valueClosed chan struct{}
	// graceTimer is the armed AfterFunc; arming is the window between the orphan
	// log and that arming, so grace never starts before the orphan line lands.
	graceTimer *time.Timer
	arming     bool
	graceGen   uint64
	logger     *slog.Logger
}
```

| Field | Type | Purpose |
| --- | --- | --- |
| `value` | `any` | The stored value the caller type-asserts. Written once at construction. |
| `cancel` | `context.CancelFunc` | Cancels this incarnation's lifetime context (`life`), which the lifetime goroutine watches. |
| `holders` | `map[uint64]struct{}` | Set of live holder ids. Empty ⇒ orphaned. |
| `nextID` | `uint64` | Monotonic holder-id allocator; never reused. |
| `grace` | `time.Duration` | This incarnation's own grace, fixed by the creating `Open`. Effectively immutable after publication (written in the composite literal under `t.mu`, read only under `t.mu`). |
| `valueClosed` | `chan struct{}` | Closed by the lifetime goroutine after `stopValue` returns. `fire`/`Reset` receive on it before logging dispose. Every slot must have one (never nil). |
| `graceTimer` | `*time.Timer` | The armed `time.AfterFunc`; `nil` when not armed. |
| `arming` | `bool` | True in the window between writing `reclaim_orphan` and arming the timer, so `drop`/`bind` can see "already on the way out" while the mutex is released for the log line. |
| `graceGen` | `uint64` | Generation counter. A queued `fire` no-ops unless `graceGen` still equals the generation it was armed with. Bumped by reclaim, by a new orphan, and by `Reset`. |
| `logger` | `*slog.Logger` | The logger of the **last** `Open` that bound this key; used for orphan and dispose. Overwritten on every bind. |

## 1.5 Exported symbols and signatures (fix-reclaim `table.go`)

```go
const DefaultGrace = 10 * time.Second
const TableGrace time.Duration = -1
const MsgPut     = "reclaim_put"
const MsgBind    = "reclaim_bind"
const MsgOrphan  = "reclaim_orphan"
const MsgReclaim = "reclaim_reclaim"
const MsgDispose = "reclaim_dispose"

type Table struct{ /* unexported fields */ }

func NewTable(grace time.Duration) *Table
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, grace time.Duration, create func() (any, error)) (any, error)
func (t *Table) Reset()
```

Unexported helpers: `requireContext(ctx)`, `waitCtx(ctx)`, `stopValue(value any)`, `(*slot).gracePending()`, `(*Table).bindLocked(e *slot) (id uint64, reclaimed bool)`, `(*Table).logBind(logger, key, reclaimed)`, `(*Table).watch(key string, id uint64, e *slot, ctx context.Context)`, `(*Table).drop(key string, id uint64, e *slot)`, `(*Table).fire(key string, e *slot, gen uint64)`.

## 1.6 Optional interfaces and how they are asserted

Exactly one optional interface, unexported:

```go
// closer is an optional Close on a stored value, called when the incarnation ends.
type closer interface {
	Close()
}

// stopValue calls Close if value has it. Used for a lost create and when an incarnation ends.
func stopValue(value any) {
	if c, ok := value.(closer); ok {
		c.Close()
	}
}
```

Note the signature: `Close()` with **no error return** — deliberately not `io.Closer`, so a Traefik-style `Close() error` would *not* match. The assertion is a plain comma-ok type assertion inside `stopValue`; there is no other type switch or assertion in the package. `stopValue` is called from exactly two places: the lifetime goroutine (`go func(){ waitCtx(life); stopValue(created); close(e.valueClosed) }()`) and the lost-create-race branch of `Open` (inline, synchronous, no dispose line).

Values with no `Close()` still get a full lifecycle and a `reclaim_dispose` line (pinned by `TestTable_ValueWithoutCloseStillDisposes` with a bare `string` value).

## 1.7 Control flow

### `NewTable(grace)`
`grace < 0 → DefaultGrace`. Returns `&Table{grace, items: map[string]*slot{}}`.

### `Open(ctx, key, logger, grace, create)`
1. `t == nil` → `fmt.Errorf("reclaim: open %q: nil table", key)`.
2. `logger == nil` → `fmt.Errorf("reclaim: open %q: nil logger", key)`.
3. `requireContext(ctx)` — panics `"reclaim: Open requires a context"` if `ctx == nil`.
4. **Fast path (existing slot).** Lock; if `t.items[key]` exists: `bindLocked(e)` → `(id, reclaimed)`; overwrite `e.logger = logger`; snapshot `stored := e.value`; unlock; `logBind(logger, key, reclaimed)`; `go t.watch(key, id, e, ctx)`; return `stored, nil`. **`create` never runs, `grace` argument is discarded.**
5. **Create outside the lock** so two first Opens can race: `life, cancel := context.WithCancel(context.Background())`, then `created, err := create()`. On error: `cancel()`, return `nil, err` — no slot stored, no log line.
6. **Lock again. Lost-race branch:** if `t.items[key]` now exists, bind onto the winner exactly as in step 4, then `cancel()` this create's lifetime and `stopValue(created)` **inline** (synchronous, on the caller's goroutine), log bind, spawn watch, return the *stored* value. The loser emits no `reclaim_put` and no `reclaim_dispose`.
7. **First-put branch:** `ownGrace := grace; if ownGrace < 0 { ownGrace = t.grace }`. Build the slot with `valueClosed: make(chan struct{})`, `logger: logger`. `t.items[key] = e`; `id, _ := t.bindLocked(e)`; unlock.
8. Start the **lifetime goroutine**: `go func(){ waitCtx(life); stopValue(created); close(e.valueClosed) }()`.
9. `logger.Debug(MsgPut, "key", key)`, then `logBind(logger, key, false)`.
10. Start the **watch goroutine**: `go t.watch(key, id, e, ctx)`. Return `created, nil`.

**Two goroutines per key plus one watch goroutine per additional `Open`.** Precisely: one lifetime goroutine per incarnation, one watch goroutine per successful `Open` call.

### `watch(key, id, e, ctx)`
```go
waitCtx(ctx)
t.drop(key, id, e)
```
`waitCtx` prefers `<-ctx.Done()`; if `Done()` returns nil (Yaegi/`context.Background`), it **polls `ctx.Err()` every 20 ms**. A `context.Background()` holder therefore parks a goroutine that polls forever — called out in the card's "Rank-up moves".

### `drop(key, id, e)` — the arming window
1. Lock. If `t.items[key]` is missing or is a different slot pointer → unlock, return (stale watcher from `Reset` or an older incarnation).
2. `delete(e.holders, id)`. If holders remain **or** `e.gracePending()` (timer armed *or* `arming`) → unlock, return.
3. Last holder gone: `e.graceGen++`; `e.arming = true`; snapshot `orphanLog := e.logger`; **unlock**.
4. `orphanLog.Debug(MsgOrphan, "key", key)` — outside the mutex, and **before** grace starts. This is what makes orphan always precede dispose.
5. Lock again. `e.arming = false`. If `t.items[key] != e` **or** `len(e.holders) > 0` → unlock, return (someone reclaimed with a live holder while the orphan line was being written).
6. Re-read `gen := e.graceGen` **at re-lock rather than trusting the value from step 3** — this is the fix for the "stranded incarnation" defect (see §6). An `Open` may have reclaimed and released inside the window, and its own `drop` returned early on `arming`, so this drop still owns the arm.
7. If `e.grace == 0`: unlock, then call `t.fire(key, e, gen)` **inline on this watcher goroutine** — so a slow `Close()` blocks the drop rather than a timer.
8. Otherwise `e.graceTimer = time.AfterFunc(e.grace, func(){ t.fire(key, e, gen) })`; unlock.

### `bindLocked(e)` (caller holds `t.mu`)
```go
if e.gracePending() {              // graceTimer != nil || arming
	if e.graceTimer != nil { e.graceTimer.Stop(); e.graceTimer = nil }
	e.graceGen++
	reclaimed = true
}
e.nextID++
e.holders[e.nextID] = struct{}{}
return e.nextID, reclaimed
```
The generation bump is what tells a queued `fire`, **and a drop that is still arming**, to leave this slot alone.

### `fire(key, e, gen)` — dispose
1. Lock. Bail unless all four hold: key mapped, `cur == e`, `e.graceGen == gen`, `len(e.holders) == 0`.
2. Snapshot `cancel := e.cancel`, `disposeLog := e.logger`; `delete(t.items, key)`; unlock.
3. `cancel()` (if non-nil) → wakes the lifetime goroutine → it runs `stopValue(value)` → `close(e.valueClosed)`.
4. **`<-e.valueClosed`** — blocks until `Close()` has returned.
5. `disposeLog.Debug(MsgDispose, "key", key)`.

This ordering is the headline guarantee of the branch: the dispose line is a *completion* signal.

### `Reset()` — tests only
1. `t == nil` → return.
2. Lock. Steal `items`, install a fresh empty map. **Under the same lock**, for every stolen slot: stop and nil `graceTimer`, set `arming = false`, `graceGen++` (invalidating queued fires and arming drops). Unlock.
3. Then, outside the lock, per slot: `e.cancel()`, `<-e.valueClosed`, `e.logger.Debug(MsgDispose, "key", key)`.

`Reset` is therefore synchronous: when it returns, every value's `Close()` has returned and every dispose line has been written.

---

# 2. `pkg/reclaim/default.go` on fix-reclaim (48 lines)

```go
var (
	defaultMu    sync.Mutex
	defaultTable *Table
)

func Default() *Table
func Open(ctx context.Context, key string, logger *slog.Logger, grace time.Duration, create func() (any, error)) (any, error)
func Reset()
func ResetWith(grace time.Duration)
```

- `Default()` — lazy singleton under `defaultMu`; first use builds `NewTable(DefaultGrace)`.
- `Open(...)` — one-line façade: `return Default().Open(ctx, key, logger, grace, create)`. The `grace` pass-through is the thing `TestDefault_OpenForwardsTheGrace` was written to pin (a mutation that dropped the argument survived the whole suite before that test existed).
- `Reset()` — `ResetWith(DefaultGrace)`.
- `ResetWith(grace)` — under `defaultMu`: `defaultTable.Reset()` if non-nil, then `defaultTable = NewTable(grace)`. Blocks until every stored value's `Close()` has returned, so tests can use it as teardown that really stops the tickers it started.

Doc comment worth carrying forward: *"`Close()` must not block."*

---

# 3. `pkg/reclaim/table_test.go` on fix-reclaim — all 47 tests

## 3.1 Test-file fixtures and helpers (rewritable from this section alone)

**Constants**
```go
const waitBudget = 10 * time.Second   // guards a condition that should already be true
const graceNoRace = 5 * time.Second   // long enough that a reclaim-branch assert cannot lose the timer race
```

**Disposable stand-in values**
```go
type box struct { n int; ended *atomic.Bool }
func (b *box) Close() { if b.ended != nil { b.ended.Store(true) } }
func ending(n int, done *atomic.Bool) *box { return &box{n: n, ended: done} }

type namedEnd struct { name string; mu *sync.Mutex; ended *[]string }
func (n *namedEnd) Close() { n.mu.Lock(); *n.ended = append(*n.ended, n.name); n.mu.Unlock() }

type counterClose struct { closes atomic.Int32 }
func (c *counterClose) Close() { c.closes.Add(1) }

type closeProbe struct {
	handler *recHandler; key string
	closed atomic.Bool; sawDispose atomic.Bool
}
func (p *closeProbe) Close() {
	p.sawDispose.Store(countKeyMsg(p.handler.events(), MsgDispose, p.key) > 0)
	p.closed.Store(true)
}
```
`closeProbe` is the instrument for the whole "dispose follows Close" guarantee: it records, *from inside `Close()`*, whether the dispose line for its key had already been written.

**Log capture**
```go
type recHandler struct { mu sync.Mutex; recs []slog.Record }
func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }   // keeps debug
func (h *recHandler) Handle(_ context.Context, r slog.Record) error             // appends r.Clone() under mu
func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recHandler) WithGroup(string) slog.Handler { return h }
func (h *recHandler) events() [][2]string   // {msg, key} per record, in order; key read from the "key" attr
func (h *recHandler) requireLevels(t *testing.T)  // every recorded line whose Message is one of the five must be slog.LevelDebug
```

**Level gate** (for the logger-level test)
```go
type levelGate struct { min slog.Level; recHandler }
func (h *levelGate) Enabled(_ context.Context, l slog.Level) bool { return l >= h.min }
```
Note it *embeds* `recHandler`, so tests reach the recorder as `&debugH.recHandler`.

**Ordering gate** (for the arming-window tests) — the single most important fixture to preserve if any lifecycle ordering survives the redesign:
```go
type gateHandler struct {
	recHandler
	gate    string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}
// Handle records the line, then blocks the FIRST line whose Message == gate:
//   h.once.Do(func(){ close(h.entered); <-h.release })
```
Used as `&gateHandler{gate: MsgOrphan, entered: make(chan struct{}), release: make(chan struct{})}`; the test does `<-h.entered`, performs work inside the window, then `close(h.release)`.

**Sequence/count assertions**
```go
func keySeq(ev [][2]string, key string) []string            // messages for one key, in order
func countKeyMsg(ev [][2]string, msg, key string) int
func countMsg(ev [][2]string, msg string) int               // across every key
```

**Waiting**
```go
func waitUntil(t *testing.T, cond func() bool)               // polls 1 ms, t.Fatal("timeout waiting for condition") after waitBudget
func waitKeyMsg(t *testing.T, h *recHandler, msg, key string)
func requireDisposeAfterClose(t *testing.T, h *recHandler, key string, closed *atomic.Bool)
// ^ waits for the DISPOSE LINE first, then asserts closed.Load(). The flag is set first,
//   so waiting on the flag and then reading the log races it. This is the documented rule.
```

**White-box slot access**
```go
func mustSlotA(t *testing.T, tab *Table) *slot   // tab.mu.Lock(); tab.items["a"]; Fatal if nil
```
Many tests then re-lock `tab.mu` to read `e.graceTimer`, `e.grace`, `e.graceGen`, `e.arming`, `len(e.holders)`, `len(tab.items)`.

**Goroutine baseline**
```go
func settledGoroutines() int { time.Sleep(20 * time.Millisecond); return runtime.NumGoroutine() }
// used as: base := settledGoroutines(); ... ; waitUntil(t, func() bool { return runtime.NumGoroutine() <= base+slack })
// with const slack = 2 (was 8 before the deterministic join landed) and const keys = 50
```

**Yaegi-shaped holder context** (`Done()` returns nil, must be polled)
```go
type nilDoneCtx struct { canceled atomic.Bool }
func (c *nilDoneCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *nilDoneCtx) Done() <-chan struct{}       { return nil }
func (c *nilDoneCtx) Err() error                  { if c.canceled.Load() { return context.Canceled }; return nil }
func (c *nilDoneCtx) Value(any) any               { return nil }
func (c *nilDoneCtx) cancel()                     { c.canceled.Store(true) }
```

Test-file imports: `context, errors, go/parser, go/token, log/slog, reflect, runtime, strconv, strings, sync, sync/atomic, testing, time`.

## 3.2 (a) Tests that survive a redesign adding sleep/wake and removing per-slot context/goroutine/`arming`

These pin externally observable behaviour of `Open`/grace/keys/logging that a redesign should still honour.

| Test | Behaviour pinned |
| --- | --- |
| `TestTable_OpenCancelDispose` | Cancel + grace ends the incarnation exactly once; the per-key sequence is `put, bind, orphan, dispose`; dispose is not logged before `Close()` ran; dispose came no earlier than 3/4 of grace; all five lines are debug. |
| `TestTable_OpenDuringGraceReclaims` | An `Open` after `reclaim_orphan` and before grace ends returns the *same pointer*, does not end the incarnation, and produces `put, bind, orphan, reclaim, bind`. (Also reaches into `e.graceTimer` — see (b).) |
| `TestTable_SecondCreateDisposeIgnored` | A second `Open` on a live key does not run `create` (`created == 1`) and does not replace the lifetime. |
| `TestTable_TwoOpensOneDispose` | One live holder keeps the value alive; only after both contexts are Done does it end. |
| `TestTable_NegativeGraceUsesDefault` | `NewTable(-1)` ⇒ `tab.grace == DefaultGrace`. |
| `TestTable_GraceIsPerIncarnation` | A key opened with grace `0` ends immediately while a key opened with `TableGrace` on the same table (built with `2*waitBudget`) is still alive — grace belongs to the key, not the table. |
| `TestTable_KeyGraceSurvivesAZeroGraceTable` | The reverse direction: a key opened with `graceNoRace` on `NewTable(0)` must not take the table's zero grace. Pins `drop`'s inline zero-grace branch reading `e.grace`, not `t.grace`. |
| `TestTable_OpenGraceBelowZeroTakesTheTable` | `TableGrace` is a spelling, not a magic number: `Open(..., -time.Hour, ...)` stores the table's grace on the slot. |
| `TestTable_ReclaimDoesNotChangeTheGrace` | An `Open` that reclaims with a different grace cannot shorten or extend the incarnation's grace. |
| `TestTable_ZeroGraceEndsImmediately` | Zero grace ends the incarnation as soon as the last holder is Done; sequence `put, bind, orphan, dispose`. |
| `TestTable_StdlibImports` | `table.go` and `default.go` parse (ImportsOnly) with no import path containing a `.` — i.e. stdlib only. |
| `TestTable_HashChangeProof` | Canceling key A does not cancel a live key B; only A's `namedEnd` fires; A's sequence is `put, bind, orphan, dispose`; B never disposes and has exactly one put. |
| `TestDefault_OpenSharesIncarnation` | Package `Open` and `Default().Open` hit the same table; second create is not run (`n == 7`). |
| `TestTable_OpenNilContextPanics` | `Open(nil, ...)` panics. |
| `TestTable_OpenBackgroundDoesNotPanic` | `context.Background()` is accepted as a holder. |
| `TestTable_CreateErrorCancelsLife` | A failed `create` stores nothing and logs no put; a retry `Open` then creates successfully. |
| `TestTable_LostCreateRaceCancelsLoser` | Two concurrent first `Open`s gated on a channel: both return the same value, exactly one `reclaim_put`, and **exactly one** of the two created values was closed (`closed[0] != closed[1]`). |
| `TestTable_ResetLogsDisposeAndKeepsNextIncarnation` | `Reset` logs one dispose; a later `Open` of the same key makes a new incarnation that a stale holder drop cannot cancel. |
| `TestTable_ResetStopsArmedTimer` | `Reset` during grace stops the timer and still logs exactly one dispose. |
| `TestTable_ConcurrentOpenSameKeySharesOneIncarnation` | 8 parallel `Open`s on one key return one value; after all cancels, exactly one dispose. |
| `TestTable_NilTableOpenErrors` | `(*Table)(nil).Open` returns an error rather than panicking. |
| `TestTable_NilOpenLoggerRejected` | `Open` with `nil` logger returns an error. |
| `TestTable_ConcurrentCancelLastHolders` | 8 holders canceled concurrently produce exactly one orphan and one dispose. |
| `TestTable_ResetNil` | `(*Table)(nil).Reset()` is a no-op. |
| `TestTable_OpenLoggerLevelGatesPutDispose` | An info-level `Open` logger suppresses `reclaim_put`/`reclaim_dispose`; a debug one shows both. |
| `TestTable_DisposeLogFollowsClose` | Grace path: at the moment `reclaim_dispose` is observed, `Close()` has returned, **and** `Close()` did not see a dispose line already written. |
| `TestTable_ResetDisposeLogFollowsClose` | Same on the `Reset` path, synchronously: `Reset` does not return before `Close()` ran and exactly one dispose was logged. |
| `TestTable_HolderWithoutDoneChannelIsPolled` | A holder with `Done() == nil` keeps the incarnation until `Err()` is set, then it disposes. **Redesign note:** this is the polling branch; a sleep/wake design still needs a story for Yaegi contexts. |
| `TestTable_ValueWithoutCloseStillDisposes` | A stored value with no `Close()` (a bare `string`) still ends and still logs dispose. |
| `TestDefault_ResetWithAppliesGrace` | `ResetWith(37ms)` installs that grace; `Reset()` restores `DefaultGrace`. |
| `TestDefault_OpenForwardsTheGrace` | The package façade passes the caller's `grace` through (table grace set to `2*waitBudget` so only the key's own `0` can explain the dispose). |
| `TestDefault_ConcurrentFirstUseReturnsOneTable` | 16 concurrent `Default()` callers all get the same `*Table`. |
| `TestTable_RepeatedReclaimCyclesKeepOneIncarnation` | 5 orphan→reclaim cycles keep one incarnation: `create` ran once, one put, one dispose, exactly 5 `reclaim_reclaim`; `Reset` closes the value synchronously. |
| `TestTable_HolderIdsAreReleased` | 20 short-lived holders on a key with one keep-alive holder leave `len(e.holders) == 1` — no id leak. |
| `TestTable_OrphanAndDisposeUseTheLastBindingLogger` | Orphan and dispose go through logger two; logger one records neither, but keeps its own put and bind. |
| `TestTable_ManyKeysDisposeIndependently` | 20 keys, one dropped: exactly one dispose across all keys and `len(tab.items) == keys-1`. |
| `TestTable_ResetRacingOpenClosesEveryValue` | 50 rounds of `Open` racing `Reset`: **every** created value closed **exactly once** (checked per value, not in aggregate, because a total would be satisfied by one double-close plus one never-closed), and a reset table stores nothing. |
| `TestTable_GoroutinesReturnToBaseline` | 50 keys opened and ended return `runtime.NumGoroutine()` to `base+2`. **Redesign note:** this becomes trivially true and much stronger if the per-slot goroutines go away; keep it as a regression guard on whatever replaces them. |
| `TestTable_ReclaimRacesFire` | 40 rounds at 3 ms grace: either side of the grace edge is correct, but a reclaimed incarnation must survive the queued `AfterFunc` (and must not re-run `create`), a replaced one must have had its value closed, and the returned value must always be the *stored* one. |
| `TestTable_ZeroGraceOpenRacesCancel` | 40 rounds of `Open` racing the last-holder fire at grace 0: never drops a live holder; the returned value is always the stored incarnation. |

## 3.3 (b) Tests that pin an implementation detail that is going away

These reach into `slot` internals, the per-slot goroutine model, or the `arming` flag. A redesign that removes the per-slot context/goroutine/`arming` must delete or rewrite them.

| Test | Implementation detail it pins |
| --- | --- |
| `TestTable_BindWhileArmingReclaims` | **Pure white-box.** Sets `e.arming = true` by hand under `tab.mu`, calls `tab.bindLocked(e)` directly, asserts `reclaimed == true` and that `graceGen` was bumped. Meaningless without `arming`. |
| `TestTable_ReclaimAndReleaseInsideOrphanWindowStillArms` | Drives the exact interleaving the `arming` window created: gate the orphan line, reclaim with an *already-Done* holder so its own drop returns early on `arming`, then assert the held drop still arms and the key is finally evicted rather than stranded. This is the regression test for a defect that only exists because `arming` exists. |
| `TestTable_OpenInsideOrphanWindowKeepsIncarnationLive` | Also gate-driven through `drop`: after releasing the gate it `waitUntil`s on `!e.arming`, then reads `e.graceTimer`, `len(e.holders)`, and `tab.items["a"]` under the mutex. Asserts the exact key string `"reclaim_put,reclaim_bind,reclaim_orphan,reclaim_reclaim,reclaim_bind"`. |
| `TestTable_StaleFireAfterReclaimNoops` | Calls the unexported `tab.fire("a", e, gen)` with a captured stale `e.graceGen`. |
| `TestTable_StaleFireWhileHeldNoops` | Calls `tab.fire("a", e, e.graceGen)` while a holder is live. |
| `TestTable_StaleFireAfterResetNoops` | Captures an old slot pointer + generation across a `Reset`, then calls `tab.fire` on it. |
| `TestTable_OpenDuringGraceReclaims` (partly) | In addition to its black-box asserts, it reads `e.graceTimer != nil` under `tab.mu` to prove the timer was *stopped*, not merely slow. The behavioural half survives; the `graceTimer` probe does not. |
| `TestTable_OpenGraceBelowZeroTakesTheTable`, `TestTable_ReclaimDoesNotChangeTheGrace` (partly) | Both read `e.grace` off the slot under `tab.mu`. The grace *semantics* survive; the field probe needs a new home. |
| `TestTable_GoroutinesReturnToBaseline` (partly) | Its comment and `slack = 2` are calibrated to "two goroutines per key are joined deterministically". The number changes; the guard should stay. |
| `TestTable_HolderIdsAreReleased` (partly) | Polls `len(e.holders)` directly. |
| `TestTable_HolderWithoutDoneChannelIsPolled` | Exists because `waitCtx` polls `Err()` every 20 ms on a nil-`Done` holder. If watchers go away, this needs a new mechanism or the case needs a new answer. |
| `TestTable_ManyKeysDisposeIndependently`, `TestTable_ResetRacingOpenClosesEveryValue` (partly) | Read `len(tab.items)` under `tab.mu`. |
| `TestTable_NegativeGraceUsesDefault`, `TestDefault_ResetWithAppliesGrace` | Read `tab.grace` / `Default().grace` directly. |

## 3.4 (c) Tests that pin log-ordering guarantees

| Test | Ordering pinned |
| --- | --- |
| `TestTable_OrphanPrecedesDisposeAtTinyGrace` | **200 rounds** at `NewTable(time.Nanosecond)` — the timer fires essentially as it is armed. Requires `keySeq == [put, bind, orphan, dispose]` every round. On master's `table.go` this recorded `[put, bind, dispose, orphan]` on round 57 of 300. |
| `TestTable_DisposeLogFollowsClose` | `reclaim_dispose` is emitted strictly after `Close()` returned (grace path), and `Close()` must not observe an already-written dispose line. |
| `TestTable_ResetDisposeLogFollowsClose` | Same on the `Reset` path, and `Reset` blocks until it is true. |
| `TestTable_OpenCancelDispose` | Exact per-key sequence `put, bind, orphan, dispose` + one dispose + all-debug levels. |
| `TestTable_ZeroGraceEndsImmediately` | Same exact sequence at grace 0. |
| `TestTable_OpenDuringGraceReclaims` | Exact sequence `put, bind, orphan, reclaim, bind`. |
| `TestTable_OpenInsideOrphanWindowKeepsIncarnationLive` | Exact joined string `reclaim_put,reclaim_bind,reclaim_orphan,reclaim_reclaim,reclaim_bind`, with the gate holding the orphan line until Open 2 has bound. |
| `TestTable_HashChangeProof` | A's sequence `put, bind, orphan, dispose` with B interleaved and never disposing. |
| `TestTable_ConcurrentCancelLastHolders` | Exactly one orphan and one dispose despite 8 concurrent cancels. |
| `TestTable_OrphanAndDisposeUseTheLastBindingLogger` | *Which logger* each line lands on (ownership, not sequence). |
| `recHandler.requireLevels` (called by 12 tests) | Every one of the five messages is at `slog.LevelDebug`. |

---

# 4. `knowledge/devdocs/std_go_reclaim.md` on fix-reclaim (67 lines)

## 4.1 Ubiquitous-language terms defined

Six terms, each with an `_Avoid_:` line:

1. **Table** — a keyed store of `any` values plus holder contexts; the value stays if a new context opens the same key before grace ends, otherwise the incarnation lifetime is canceled; the caller type-asserts. *Avoid:* `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`.
2. **Default** — the process-wide table (`reclaim.Default`, `reclaim.Open`), one incarnation per key for the whole process. *Avoid:* one `NewTable` per caller when they should share; unprefixed keys that can collide.
3. **Open** — create-once for a key; `create` takes no args (Yaegi assigns a `context.Context` arg onto the value); caller passes a required `*slog.Logger` (no table logger, no fallback) and a `grace` that only takes effect when this call's value becomes the stored one; a later `Open` does not run create; `Close()` is called when the incarnation ends; `ctx` must not be nil. *Avoid:* Put vs Bind as two public calls; a nil holder context; `func(context.Context) (any, error)` as create.
4. **Incarnation** — one instance of a value under a key: begins at the `Open` that found the key absent and ran `create`, ends when its lifetime is canceled. A reclaim keeps the same incarnation (same pointer); a create after dispose is a different one. Log lines carry the key, not an instance id, so two incarnations of a key are told apart by the `reclaim_put` between them. *Avoid:* "the entry"/"the key" when you mean this instance; treating a reclaimed value as a new one.
5. **Grace** — wait after the last bound context is Done, before the lifetime is canceled; an `Open` in that window is a reclaim; zero means no wait; **grace belongs to the incarnation** (the creating `Open` fixes it, a reclaiming `Open` cannot change it); in a create race the winner's grace applies; `TableGrace` (any negative duration) takes the table's; it is an `Open` argument only — a negative grace at `NewTable` is `DefaultGrace` (10s). *Avoid:* passing `0` when you meant the table's; expecting a reclaiming `Open`'s grace to take effect.
6. **Lifetime** — the per-incarnation context the table holds; canceled when the incarnation ends (grace elapsed while orphaned, `Reset`, or a lost create race); a goroutine started by `Open` watches it and calls `Close()` on the value; `create` takes no arguments so callers never see this context. *Avoid:* a house `dispose func(any)` on `Open`.

The **Overview** also states the standing rule: *"This package is a shared copy."* The same `pkg/reclaim` lives in `traefik-modsecurity` and the two are synced in both directions, so edits are **additive** — fix a defect, add coverage — and never remove or reshape what the package already does, even where a piece looks unreachable from this repo. Keep it stdlib-only and self-contained so the port stays a file copy. Diff against upstream `main` before changing it. **This rule is the main constraint on any redesign.**

## 4.2 Gotchas section — verbatim

> ## Gotchas
>
> - Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`). A zero grace — `NewTable(0)` for every key that names none, or `Open(…, 0, …)` for one key — ends the incarnation as soon as the last holder is gone. `0` in the `Open` slot is the most aggressive value, not the default; the default is `TableGrace`.
> - Yaegi: do not write `Table[*T]` on a type from another package.
> - A second `Open` while the incarnation is live or in grace does not replace the lifetime.
> - Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.
> - A flag set inside the value's `Close()` is **not** a signal that `reclaim_dispose` was logged — `Close()` runs first, then the line. A test that waits on such a flag and then reads the log races it. Wait for the line (`waitKeyMsg`) and assert the flag afterwards.
> - Canceling a holder and immediately calling `Open` again is usually *not* a reclaim: the drop runs on the holder's watcher goroutine, so the second `Open` often arrives while the first holder is still counted and is a plain second bind. A test that means to exercise the reclaim branch must wait for `reclaim_orphan` first.
> - A test must never need a specific timer to win. Either use a grace long enough that the branch you assert cannot lose (`graceNoRace`), or accept both sides of the grace edge and assert what must hold for the side that happened. A test that requires "the reclaim beat the 3 ms timer" is a CI flake, not a check.
> - `reclaim_orphan` is emitted before grace is armed, so it always precedes the `reclaim_dispose` that ends grace, at any grace. The `arming` window on the slot is what buys that ordering without logging under the table mutex. Two cases sit outside it by design: `Reset` cancels everything at once and can log dispose before an orphan line a concurrent cancel is still writing, and an `Open` that binds inside the arming window can record its `reclaim_reclaim` before that line.

Other sections worth knowing: **How to use** says production call shape is `reclaim.Open(ctx, key, logger, reclaim.TableGrace, create)`; `Reset`/`ResetWith` block until every stored value's `Close()` has returned; `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`; `Close()` must not block because it holds up the grace timer or `Reset`; prefix keys when several types share Default (`bin:` / `mmdb:` / `plugin:`).

---

# 5. `openspec/specs/std_go_reclaim_context-lease/spec.md` on fix-reclaim (162 lines)

## 5.1 Requirement headers and scenario names, verbatim

```
### Requirement: Table file depends only on the Go standard library
#### Scenario: Stdlib-only imports

### Requirement: Process table is a singleton
#### Scenario: Default Open shares one incarnation

### Requirement: Open creates once and binds a context
#### Scenario: Two holders one dispose
#### Scenario: Second create dispose is ignored
#### Scenario: Lost create race
#### Scenario: Missing context panics
#### Scenario: Nil logger is rejected

### Requirement: Cancel then open within grace does not dispose
#### Scenario: Reclaim before grace
#### Scenario: Grace elapses without rebind

### Requirement: Keys are independent
#### Scenario: One key times out

### Requirement: Grace is configurable
#### Scenario: Default grace
#### Scenario: Zero grace
#### Scenario: One key names its own grace
#### Scenario: A key outlives a table that ends its keys immediately
#### Scenario: Reclaim does not change the grace

### Requirement: Lifecycle events are logged
#### Scenario: Hash change orphan then dispose
#### Scenario: Orphan precedes dispose at a short grace
#### Scenario: Reset logs dispose
#### Scenario: Open logger level gates put and dispose
#### Scenario: Orphan and dispose use the last binding logger

### Requirement: Incarnation end closes the stored value before it reports the end
#### Scenario: Dispose log implies Close has returned
#### Scenario: Reset closes the value before it reports dispose
#### Scenario: Goroutines do not outlive the incarnation

### Requirement: Either side of the grace edge is correct
#### Scenario: Open racing grace expiry
```

Eight requirements, 22 scenarios.

## 5.2 The two ordering guarantees, in full

**(a) `reclaim_dispose` only after `Close()` returns** — the body of *Requirement: Incarnation end closes the stored value before it reports the end*, verbatim:

> When an incarnation ends (grace elapsed while orphaned, `Reset`, or a lost create race), the table SHALL cancel the incarnation lifetime and then wait until the stored value's `Close()` has returned before it reports the end. For the two paths that report it — grace elapsed and `Reset` — the table SHALL emit `reclaim_dispose` only after that `Close()` has returned. A lost create race closes the value it created inline and emits no `reclaim_dispose`, because that value was never stored. `Close()` SHALL be called at most once per incarnation. Because the table waits, a value whose `Close()` blocks blocks whoever ended the incarnation; values stored on this table SHALL NOT block in `Close()`. Every goroutine the table starts for a key SHALL exit once that key's holder contexts are Done and its incarnation has ended.

with scenarios:

> #### Scenario: Dispose log implies Close has returned
> - **WHEN** a key is orphaned and grace elapses
> - **AND** the stored value has a `Close()` method
> - **THEN** `Close()` has returned before `reclaim_dispose` is emitted for that key
>
> #### Scenario: Reset closes the value before it reports dispose
> - **WHEN** `Reset` is called on a table that still has an incarnation
> - **THEN** that value's `Close()` has returned before `reclaim_dispose` is emitted for that key
> - **AND** `Reset` does not return before that dispose is emitted
>
> #### Scenario: Goroutines do not outlive the incarnation
> - **WHEN** many keys are opened, then every holder context is Done and every incarnation has ended
> - **THEN** the table owns no more goroutines than it did before those `Open` calls

**(b) `reclaim_orphan` always precedes `reclaim_dispose`** — from the body of *Requirement: Lifecycle events are logged*, verbatim (final sentences):

> Log lines MUST NOT be emitted while the table mutex is held. `reclaim_orphan` SHALL be emitted before the grace period starts. So for one incarnation `reclaim_orphan` always precedes the `reclaim_dispose` that ends grace, at every grace duration including zero, and always precedes the `reclaim_reclaim` of an `Open` that lands in that grace window. Two narrow cases are outside that ordering, and both are stated so a reader does not treat them as defects: a `Reset` cancels every mapped incarnation at once, so it MAY emit `reclaim_dispose` before an orphan line that a concurrent last-holder cancel has not finished writing; and an `Open` that binds in the instant between the last holder going and the orphan line being written MAY record its `reclaim_reclaim` before that line.

with the dedicated scenario:

> #### Scenario: Orphan precedes dispose at a short grace
> - **WHEN** a table with a grace shorter than a millisecond has its last holder for a key go Done
> - **AND** the grace elapses and the lifetime is canceled
> - **THEN** the recorded order for that key is `reclaim_orphan` then `reclaim_dispose`

Two other requirement bodies matter for a redesign:

- *Either side of the grace edge is correct*: "An `Open` that races the end of grace for the same key SHALL either reclaim the stored incarnation (returning the stored value and keeping the lifetime) or bind a new incarnation created by that `Open`. Both outcomes are correct. In neither outcome SHALL the table cancel the lifetime of an incarnation that has a live holder, and in neither outcome SHALL a value be left stored after its lifetime was canceled."
- *Grace is configurable* now begins "Grace SHALL belong to the incarnation, not to the table."

---

# 6. `devstate/2026/09/2026-09-08-fix-reclaim/card.md` — Findings and Axis review

Header: *"Developer review: ready to merge — 2026-09-08T17:08:23Z"*. Reviewed head `ec6db3e`. Tests in `pkg/reclaim`: **26 → 47**. Axis findings: **40 found / 35 applied / 5 argued / 0 open**, across two Opus rounds. Priority P2. CI run 34254850208 green on `ec6db3e` (Test, Lint, Integration Tests).

## 6.1 The three real defects (as opposed to style)

### Defect 1 — the arming window this change added could strand an incarnation forever
- **Where:** `pkg/reclaim/table.go`, `drop`.
- **What it was:** the fix for orphan-ordering introduced `slot.arming`, the window between writing `reclaim_orphan` and arming the grace timer. A `drop` that entered that window released the mutex to log. If, during that window, an `Open` reclaimed the slot (bumping `graceGen`) and *its own already-Done holder* immediately dropped, that second `drop` returned early because `e.gracePending()` was still true (`arming` set). The original `drop` then re-locked, compared against the generation it had captured *before* logging, found it moved, and bailed.
- **How it manifested:** the slot stayed mapped with **no holders, no timer, and the value's `Close()` never run** — a permanent leak of a database handle plus its update ticker. Silent; nothing logged.
- **How it was fixed:** `drop` now re-reads the state at re-lock rather than trusting what it captured before the log line: `e.arming = false`, re-check `t.items[key] == e && len(e.holders) == 0`, and only then `gen := e.graceGen`. That drop owns the arm. Test: `TestTable_ReclaimAndReleaseInsideOrphanWindowStillArms`, which uses `gateHandler` to block on the `reclaim_orphan` line and drives the exact interleaving; it **times out on the pre-fix code**.
- **Found by:** the Performance axis, by reading — no test found it.

### Defect 2 — two `pkg/dbwrappers` tests were never taking the reclaim branch they are named for
- **Where:** `TestOpenBIN_SameHashReclaimKeepsTicker`, `TestOpenMMDB_SameHashReclaimKeepsTicker`.
- **What it was:** both cancel the first holder and call `Open` again immediately, then assert pointer equality. But the drop runs on the holder's *watcher goroutine*, so the second `Open` almost always arrived while the first holder was still counted — a plain second bind, not a reclaim.
- **How it manifested:** the recorded event stream was `put, bind, bind` — no `reclaim_orphan`, no `reclaim_reclaim`. The pointer-equality assertion was trivially true, so the tests passed while testing nothing. Surfaced only once the coverage axis asked for an event recorder to be wired in.
- **How it was fixed:** both now wait for `reclaim_orphan` first and then assert the `reclaim_reclaim` line. Runtime fell to 0.05 s. The trap is written into the devdocs gotchas ("Canceling a holder and immediately calling `Open` again is usually *not* a reclaim").

### Defect 3 — the test named after per-slot grace could not fail for that behaviour
- **Where:** `TestTable_GraceIsPerIncarnation`.
- **What it was:** the test's table was built with `graceNoRace` (5 s) while `waitBudget` is 10 s. Waiting for the zero-grace key's dispose was therefore satisfied by the *table's* grace elapsing at 5 s, regardless of whether grace was per-key at all.
- **How it manifested:** reverting `drop` to a table-wide grace still passed the test in 5 s. A green test for a behaviour that had been removed.
- **How it was fixed:** the table is now built with `2 * waitBudget`, so the revert fails at the dispose wait. Two further signature-preserving mutations that had survived the entire suite now die: (i) `drop`'s inline zero-grace branch reading `t.grace` instead of `e.grace` — killed by the new `TestTable_KeyGraceSurvivesAZeroGraceTable`; (ii) the `reclaim.Open` façade discarding its `grace` argument (a silent failure mode) — killed by the new `TestDefault_OpenForwardsTheGrace`.
- **Found by:** mutation testing, not by reading.

There is a fourth Findings row that is a *consequence* rather than a defect: making `Close()` precede the dispose line inverted the two observables and broke `TestTable_OpenCancelDispose`, `TestTable_ZeroGraceEndsImmediately`, and `TestTable_ConcurrentCancelLastHolders`, all of which had used a `Close`-side flag as a proxy for "dispose was logged". All three now wait for the line and assert the flag afterwards, which makes each a second check of the new guarantee. The rule is in the devdocs gotchas.

## 6.2 The other ~36 findings, by theme

**Round 1** — all seven axes on Opus against the diff pinned at `995849a`:

| Axis | Found (hard) | Applied | Argued | Worst finding |
| --- | --- | --- | --- | --- |
| Standards | 5 (4) | 5 | 0 | The slot's `closed` field named the slot, not the fact it carries — the exact conflation the change exists to separate. Renamed `valueClosed`. |
| Nitpicks | 5 (3) | 5 | 0 | Test names and locals describing the mechanism instead of the behaviour (`armingGen`, `useLeases`, `...DisposesQuietly`). |
| Spec | 4 | 4 | 0 | The delta spec claimed `reclaim_orphan` always precedes `reclaim_dispose` "at every grace", which `Reset` does not honour. The requirement now states both exceptions. |
| Security | 0 | — | — | None. No new input handling, no logging of untrusted data, no new lock ordering. |
| Performance | 4 (1) | 2 | 2 | Defect 1 above — the stranded incarnation. |
| Dead code | 2 | 1 | 1 | An unreachable nil-channel guard in a `waitClosed` helper; the helper was deleted and `fire`/`Reset` now receive on `valueClosed` directly. |
| Test coverage | 7 (2) | 7 | 0 | The arming window — the state this change adds — had no test that entered it through `drop`. |

**Round 2** — the per-slot grace commit `5146d0c` reviewed on its own, five axes:

| Axis | Found (hard) | Applied | Argued | Worst finding |
| --- | --- | --- | --- | --- |
| Correctness and concurrency | 0 | — | — | None. `slot.grace` is written once in the composite literal under `t.mu` before publication and both reads are under the lock, so it is effectively immutable. |
| Test coverage | 4 (1) | 4 | 0 | Defect 3 — the headline test could not fail; plus the two surviving mutations. |
| Spec and artifact truth | 4 (2) | 4 | 0 | `proposal.md` claimed callers were unaffected at the API level five lines after listing the three call sites that changed, and still counted three requirement blocks where there are now five. |
| Naming and API shape | 4 | 3 | 1 | `TableGrace` is a `time.Duration`, so `NewTable(TableGrace)` compiles and silently means 10 s — the one reading its name denies. Closed by doc comment. |
| Portability to the shared copy | 0 | — | — | None. Still stdlib-only, non-generic, `create` unchanged; the port stays a file copy plus one argument per call site. |

**The 5 argued findings** (kept, with measurements): two wall-clock trims that would have removed a regression guard (the 40 rounds first catch the defect on round 57; the `pkg/dbwrappers` sleeps cover a `pkg/dbsource` shutdown gap, now a follow-up); one pre-existing test-only production symbol; `TableGrace` kept over `InheritGrace` because it names where the value comes from; and no `NoGrace = 0` constant added because new API surface on a shared copy costs more than the devdocs line.

## 6.3 Decisions and follow-ups that bind the redesign

- **"Should the lifetime context and its per-slot goroutine be deleted, since closing inline is simpler?" — resolved by the human: no.** The no-removal rule applies even to code unreachable from this repo, so `fire`/`Reset` wait on a `valueClosed` channel instead. *This is the standing constraint that a redesign removing the per-slot context/goroutine has to reopen with the human.*
- **"Does upstream carry a sleep/wake feature?" — resolved: no.** Upstream `pkg/reclaim` is three files and a code search for `Wake`/`Sleep` in `traefik-modsecurity` returns zero hits. If such a feature exists it is in another project.
- **"Should upstream get the same fix?" — resolved by the human: yes, as a standing bidirectional-sync rule; nothing may be removed.** Written into `knowledge/devdocs/std_go_reclaim.md`.
- **Open follow-up (directly relevant):** *"`pkg/dbsource`: `Updater.Stop` does not wait for the update goroutine, and `tick` never checks for stop before downloading, so a disposed wrapper can still write into a directory `t.TempDir` is removing. That is why `settleFirstTick` exists in `pkg/dbwrappers/reclaim_test.go`. The fix is to cancel the in-flight request and join the goroutine; out of scope for a reclaim ticket."*
- **Open follow-up:** port this change to `david-garcia-garcia/traefik-modsecurity` `pkg/reclaim` (file copy of `table.go`, `default.go`, `table_test.go`).
- **Rank-up move left on the table:** *"Consider whether the holder `watch` goroutine should park forever for a `context.Background()` holder, or refuse that case outside tests."*

---

# 7. `knowledge/research/ext_traefik-modsecurity_reclaim_table/notes.md` — upstream delta

Compares sibling repo `traefik-modsecurity@645f4a2` `pkg/reclaim` against this product at `origin/master` (`d10ad52`). Same author; shared context-lease table for Traefik plugin reload.

- **`default.go` — identical.** Byte-for-byte: `Default`, package `Open`, `Reset`, `ResetWith` with `DefaultGrace`.
- **`table.go` — naming only.** Logic, constants, state machine, and public API are the same. Upstream renames locals (`created`/`stored`/`value` where this product used `v`) and the `stopValue` parameter name. No behavioural diff in `Open`, `bindLocked`, `drop`, `fire`, or `Reset`. (The fix-reclaim branch adopted upstream's local names for parity — that is the `[P3] Port the upstream variable renames` checkbox.)
- **`table_test.go` — one material delta.** Same test function set (27 `Test*` functions counted there). Upstream's `TestTable_HashChangeProof` waits for `namedEnd.Close` via `waitUntil` *after* `MsgDispose`, with a comment that `Close` runs on the life goroutine and can race the dispose log. This product's copy read `ended` immediately after `waitKeyMsg(MsgDispose)` without that wait (local lines 364–372 vs upstream 364–373). That matched the intermittent CI failure pattern. Other diffs are formatting/indentation only.
- **CI evidence gathered there:** public Actions API runs `34139964808` (2026-09-07) and `33307746472` (2026-08-30) on `master` concluded `failure` with job `Test` failed; Lint and Integration Tests succeeded. Job logs returned 403 without a token, so the reclaim-specific failure text was never verified from logs.
- **Implication recorded:** bring the upstream `HashChangeProof` wait pattern at minimum; optionally adopt upstream local naming for parity. Upstream already includes stress tests (`TestTable_ReclaimRacesFire`, `TestTable_ZeroGraceOpenRacesCancel`, stale-fire guards, concurrent paths).

**Takeaway for the redesign:** upstream is essentially the same code as this repo's master, so any redesign here creates the first real behavioural divergence between the two copies. The no-removal / bidirectional-sync rule is not hypothetical.

---

# 8. `pkg/reclaim/table.go` and `default.go` on `origin/master`

## 8.1 `table.go` on master — **260 lines**

### Constants — a single block, and no `TableGrace`
```go
const (
	DefaultGrace = 10 * time.Second

	MsgPut     = "reclaim_put"
	MsgBind    = "reclaim_bind"
	MsgOrphan  = "reclaim_orphan"
	MsgReclaim = "reclaim_reclaim"
	MsgDispose = "reclaim_dispose"
)
```
The five message string values are **identical on both branches**. Same single `"key"` attribute, same debug level.

### `Table` struct on master — identical to fix-reclaim
```go
type Table struct {
	mu    sync.Mutex
	grace time.Duration
	items map[string]*slot
}
```

### `slot` struct on master — **the list requested**
```go
type slot struct {
	value      any
	cancel     context.CancelFunc
	holders    map[uint64]struct{}
	nextID     uint64
	graceTimer *time.Timer
	graceGen   uint64
	logger     *slog.Logger
}
```

| Field | Type | Purpose |
| --- | --- | --- |
| `value` | `any` | The stored value. |
| `cancel` | `context.CancelFunc` | Cancels this incarnation's lifetime context. |
| `holders` | `map[uint64]struct{}` | Live holder ids. |
| `nextID` | `uint64` | Monotonic holder-id allocator. |
| `graceTimer` | `*time.Timer` | Armed `AfterFunc`, or nil. |
| `graceGen` | `uint64` | Generation guard so a queued `fire` no-ops after a reclaim/Reset. |
| `logger` | `*slog.Logger` | Last binding `Open`'s logger, used for orphan and dispose. |

**Seven fields.** fix-reclaim adds three: `grace time.Duration`, `valueClosed chan struct{}`, `arming bool` (ten fields total).

### Exported API on master
```go
const DefaultGrace = 10 * time.Second
const MsgPut, MsgBind, MsgOrphan, MsgReclaim, MsgDispose   // same values

type Table struct{ /* unexported */ }

func NewTable(grace time.Duration) *Table
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error)) (any, error)   // NO grace parameter
func (t *Table) Reset()
```
Unexported: `requireContext`, `waitCtx`, `stopValue`, `bindLocked`, `logBind`, `watch`, `drop`, `fire`. **No `gracePending`.** The `closer` interface and `stopValue` are identical to fix-reclaim.

### Master control flow — where it differs

`Open` is structurally the same three-branch shape (reuse / lost race / first put) minus the `grace` argument and minus `valueClosed`. The lifetime goroutine on master is:
```go
go func() {
	waitCtx(life)
	stopValue(v)
}()
```
— nothing signals when `Close()` has returned.

`bindLocked` on master keys only off the timer:
```go
if e.graceTimer != nil {
	e.graceTimer.Stop()
	e.graceTimer = nil
	e.graceGen++
	reclaimed = true
}
```

`drop` on master **arms first, logs second** — this is the ordering defect:
```go
delete(e.holders, id)
if len(e.holders) > 0 || e.graceTimer != nil { t.mu.Unlock(); return }

// Last holder gone: arm grace or end immediately when grace is zero.
e.graceGen++
gen := e.graceGen
orphanLog := e.logger
if t.grace > 0 {                                   // <-- TABLE grace, not per-slot
	e.graceTimer = time.AfterFunc(t.grace, func() { t.fire(key, e, gen) })
	t.mu.Unlock()
	orphanLog.Debug(MsgOrphan, "key", key)         // <-- logged AFTER arming
	return
}
t.mu.Unlock()
orphanLog.Debug(MsgOrphan, "key", key)
t.fire(key, e, gen)
```
With a sub-millisecond grace the timer can fire, dispose, and log before the orphan line lands — reproduced as `[put, bind, dispose, orphan]` on round 57 of 300.

`fire` on master does **not** wait for `Close()`:
```go
cancel := e.cancel; disposeLog := e.logger
delete(t.items, key)
t.mu.Unlock()
if cancel != nil { cancel() }
disposeLog.Debug(MsgDispose, "key", key)   // the life goroutine may not have run Close yet
```

`Reset` on master mutates each stolen slot **outside** the mutex and does not wait for `Close()`:
```go
t.mu.Lock(); items := t.items; t.items = map[string]*slot{}; t.mu.Unlock()
for key, e := range items {
	if e.graceTimer != nil { e.graceTimer.Stop(); e.graceTimer = nil }
	e.graceGen++
	if e.cancel != nil { e.cancel() }
	e.logger.Debug(MsgDispose, "key", key)
}
```

## 8.2 `default.go` on master — 44 lines

```go
func Default() *Table
func Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error)) (any, error)
func Reset()
func ResetWith(grace time.Duration)
```
Same lazy singleton under `defaultMu`, same `NewTable(DefaultGrace)` on first use, same `Reset()` → `ResetWith(DefaultGrace)`. The only difference from fix-reclaim is the missing `grace` parameter on `Open` and the doc comments about blocking on `Close()`.

## 8.3 Master → fix-reclaim delta, condensed

| Aspect | master | fix-reclaim |
| --- | --- | --- |
| `Open` signature | `(ctx, key, logger, create)` | `(ctx, key, logger, grace, create)` |
| Grace ownership | table-wide (`t.grace` read in `drop`) | per incarnation (`e.grace`, fixed by the creating `Open`); `TableGrace` = any negative |
| Orphan vs arm | arm, then log orphan | claim `arming`, log orphan outside the mutex, then arm |
| `slot` extra state | — | `grace`, `valueClosed`, `arming` |
| Dispose semantics | logged right after `cancel()` | logged after `<-valueClosed`, i.e. after `Close()` returned |
| `Reset` | async w.r.t. `Close()`; slot mutation outside the lock | synchronous; slot invalidation under the lock, then cancel + wait + log per slot |
| Tests | 26 | 47 |

---

# 9. `pkg/reclaim/table_test.go` on `origin/master` — 26 tests

```
TestTable_OpenCancelDispose
TestTable_OpenDuringGraceReclaims
TestTable_SecondCreateDisposeIgnored
TestTable_TwoOpensOneDispose
TestTable_NegativeGraceUsesDefault
TestTable_ZeroGraceEndsImmediately
TestTable_StdlibImports
TestTable_HashChangeProof
TestDefault_OpenSharesIncarnation
TestTable_OpenNilContextPanics
TestTable_OpenBackgroundDoesNotPanic
TestTable_CreateErrorCancelsLife
TestTable_LostCreateRaceCancelsLoser
TestTable_ResetLogsDisposeAndKeepsNextIncarnation
TestTable_ResetStopsArmedTimer
TestTable_ConcurrentOpenSameKeySharesOneIncarnation
TestTable_NilTableOpenErrors
TestTable_NilOpenLoggerRejected
TestTable_StaleFireAfterReclaimNoops
TestTable_StaleFireWhileHeldNoops
TestTable_StaleFireAfterResetNoops
TestTable_ConcurrentCancelLastHolders
TestTable_ReclaimRacesFire
TestTable_ZeroGraceOpenRacesCancel
TestTable_ResetNil
TestTable_OpenLoggerLevelGatesPutDispose
```

**Count: 26.** The 21 tests fix-reclaim adds are: the five grace-ownership tests (`GraceIsPerIncarnation`, `KeyGraceSurvivesAZeroGraceTable`, `OpenGraceBelowZeroTakesTheTable`, `ReclaimDoesNotChangeTheGrace`, `Default_OpenForwardsTheGrace`), the three ordering tests (`DisposeLogFollowsClose`, `ResetDisposeLogFollowsClose`, `OrphanPrecedesDisposeAtTinyGrace`), the three arming-window tests (`BindWhileArmingReclaims`, `ReclaimAndReleaseInsideOrphanWindowStillArms`, `OpenInsideOrphanWindowKeepsIncarnationLive`), and `GoroutinesReturnToBaseline`, `HolderWithoutDoneChannelIsPolled`, `ValueWithoutCloseStillDisposes`, `Default_ResetWithAppliesGrace`, `Default_ConcurrentFirstUseReturnsOneTable`, `RepeatedReclaimCyclesKeepOneIncarnation`, `HolderIdsAreReleased`, `OrphanAndDisposeUseTheLastBindingLogger`, `ManyKeysDisposeIndependently`, `ResetRacingOpenClosesEveryValue`.

---

# 10. `pkg/dbsource/updater.go` on `origin/master` (121 lines)

## 10.1 `Updater` struct

```go
// Updater is the keep-current loop for one source (ticker + GET).
type Updater struct {
	cfg    Config
	logger *slog.Logger
	ticker *time.Ticker
	stop   chan struct{}
}
```

| Field | Type | Purpose |
| --- | --- | --- |
| `cfg` | `dbsource.Config` | Normalized source config (Dir, Key, URL, DatabaseType, MinAge, DefaultFileName…). |
| `logger` | `*slog.Logger` | Where "database update failed" and source errors go. Defaults to `slog.Default()` in `newUpdater`. |
| `ticker` | `*time.Ticker` | The 24h ticker, created in `Start`. Nil until `Start` runs and stays nil if `!CanDownload()`. |
| `stop` | `chan struct{}` | Close-once signal that ends the select loop. Nil until `Start` runs. |

Other package-level entry points in the file: `const DefaultMinAge = 30 * 24 * time.Hour`, `WithDefaults(cfg, dir, databaseType, minAge) Config`, and `Start(cfg, logger, onUpdate) (*Updater, error)` — a free function that returns `(nil, nil)` when `strings.TrimSpace(cfg.URL) == ""`, i.e. **a nil `*Updater` means "no download configured"**, which is why every caller must nil-check.

Methods: `newUpdater` (calls `Normalize(&cfg)`, defaults the logger), `Latest() (string, error)`, `CanDownload() bool` (`u.cfg.URL != "" && u.cfg.Dir != ""`), `UpdateIfNeeded() (string, error)`, `Start(onUpdate func(path string))`, `tick(onUpdate func(path string))`, `Stop()`.

## 10.2 `Start` — how the goroutine is created and signalled

```go
func (u *Updater) Start(onUpdate func(path string)) {
	if !u.CanDownload() {
		return
	}
	u.ticker = time.NewTicker(24 * time.Hour)
	u.stop = make(chan struct{})
	go func() {
		u.tick(onUpdate)          // immediate first check, BEFORE entering the select loop
		for {
			select {
			case <-u.ticker.C:
				u.tick(onUpdate)
			case <-u.stop:
				return
			}
		}
	}()
}
```

Key points: one anonymous goroutine, no `sync.WaitGroup`, no `context.Context`, no handle returned. The **first `tick` runs outside the select loop**, so it is not guarded by `stop` at all. Signalling is one-way: close `u.stop`.

## 10.3 `tick`

```go
func (u *Updater) tick(onUpdate func(path string)) {
	path, err := u.UpdateIfNeeded()     // age check + possible HTTP GET + file write
	if err != nil {
		u.logger.Error("database update failed", "error", err)
		return
	}
	if path == "" || onUpdate == nil {
		return
	}
	onUpdate(path)                       // hot-swap callback into BIN/MMDB
}
```

`tick` reads **no** stop state whatsoever — not before `UpdateIfNeeded`, not between the download and `onUpdate`.

## 10.4 `Stop`

```go
func (u *Updater) Stop() {
	if u == nil { return }
	if u.ticker != nil { u.ticker.Stop() }
	if u.stop != nil {
		select {
		case <-u.stop:            // already closed: do nothing (idempotent)
		default:
			close(u.stop)
		}
	}
}
```

## 10.5 Precisely why `Stop` does not join, and why `tick` can download after stop

**Why `Stop` does not join the goroutine.** There is nothing to join on. `Start` throws the goroutine away — no `WaitGroup`, no `done` channel that the goroutine closes on exit, no stored `context.CancelFunc`. `u.stop` is a signal *to* the goroutine, and `Stop` never waits for a response. `Stop` returns as soon as it has stopped the ticker and closed the channel, which is potentially long before the goroutine notices. Note also that `Stop` is not itself race-safe against a concurrent `Stop` (the `select`/`default` is check-then-act, not atomic), though in practice `Close()` is called once per incarnation.

**Why `tick` can download after stop.** Three independent reasons:

1. **The first tick is unguarded.** `u.tick(onUpdate)` runs before the `for/select`. If `Stop` is called in the window between `Start` returning and that first tick completing, the tick still performs the full `UpdateIfNeeded` — HTTP GET and file write included — and still invokes `onUpdate`.
2. **A tick already in flight is never interrupted.** `stop` is only observed at the top of the select. Once `<-u.ticker.C` has been chosen and `tick` is running, closing `stop` has no effect until `tick` returns; there is no context threaded into the HTTP request, so an in-flight GET cannot be canceled.
3. **`tick` never re-checks.** Even with a stop signal pending, `tick` proceeds from `UpdateIfNeeded` straight into `onUpdate`, so the hot-swap callback can fire on a wrapper whose incarnation the reclaim table has already disposed.

**Observed consequence (from the card's follow-up):** *"a disposed wrapper can still write into a directory `t.TempDir` is removing. That is why `settleFirstTick` exists in `pkg/dbwrappers/reclaim_test.go`."* The recommended fix is to cancel the in-flight request and join the goroutine.

**Relevance to a `Sleep()`/`Wake()` design:** `Updater` is the only long-lived background worker under the wrappers, and it currently offers a one-way `Start`/`Stop` with no join and no restart. Any `Sleep()` that must *quiesce* a wrapper needs `Updater` to (a) expose a joinable shutdown, and (b) be restartable — `Start` re-creates `ticker` and `stop` each time, so calling `Start` again after `Stop` would in fact work mechanically, but it would also re-run the immediate `tick` (an unconditional age check and possible GET on every wake) and it would leak the previous goroutine if that one has not exited.

---

# 11. `pkg/dbwrappers` on `origin/master`

Both wrappers follow the same shape: a config struct, a hash-derived process-table key, an `Open*` façade that calls `reclaim.Open`, a private `new*` constructor that opens the file and starts an updater, and a `Close()` that stops the updater and the handle.

## 11.1 `bin.go` (337 lines)

**Config and struct**
```go
type BINConfig struct {
	Dir             string
	Source          dbsource.Config
	AllowMissing    bool
	DefaultFileName string
	MinAge          time.Duration
	OwnerPlugin     string `json:"-"`   // creating middleware name on BIN log lines; omitted from the share hash
	OwnerLevel      string `json:"-"`   // log level for that owner logger; omitted from the share hash
}

type BIN struct {
	cfg                BINConfig
	logger             *slog.Logger
	db                 *ip2loc.DB
	path               string
	version            *dbutils.DBVersion
	currentLocalDbCopy string
	sourceDbPath       string
	updater            *dbsource.Updater
}
```

**Key and reclaim use**
```go
const keyPrefixBIN = "bin:"
func binKey(cfg BINConfig) string { return keyPrefixBIN + cfg.Source.Key + ":" + configHash(cfg) }

func OpenBIN(ctx context.Context, cfg BINConfig, logger *slog.Logger) (*BIN, error) {
	key := binKey(cfg)
	wrap := logger
	if cfg.OwnerPlugin != "" {
		wrap = logging.NewOwner(cfg.OwnerPlugin, cfg.OwnerLevel)
	}
	v, err := reclaim.Open(ctx, key, logger, func() (any, error) {   // master: 4 args, no grace
		return newBIN(cfg, wrap)
	})
	if err != nil { return nil, err }
	w, ok := v.(*BIN)
	if !ok { return nil, fmt.Errorf("reclaim: %s: want *BIN, got %T", key, v) }
	return w, nil
}
```
Two loggers deliberately: `logger` is the *reclaim table* logger (it gets put/bind/orphan/reclaim/dispose), `wrap` is the *wrapper* logger stored on the `BIN`. `configHash` is `json.Marshal` + FNV-64a hex, so the `json:"-"` tags on `OwnerPlugin`/`OwnerLevel` keep two middlewares with different names sharing one BIN.

**Construction:** `newBIN` defaults the logger, does `logger.With("key", cfg.Source.Key)`, runs `initialize()`, and then `startUpdate()` **only if** `strings.TrimSpace(cfg.Source.URL) != ""`.
`initialize()` resolves the catalog/seed path via `dbsource.Resolve`, and if the resolved file is the *latest dated* file it makes a process-temp copy (`bin_<catalogKey>_<unixNano>.BIN` in `os.TempDir()`) and opens that instead of the source; otherwise it opens the resolved path directly. Then `ip2loc.OpenDB`, `dbutils.GetDatabaseVersion`, and an `Info` "BIN initialized" line (plus a `Warn` if the DB is more than 60 days old). With `AllowMissing` and no file it logs "no database file yet; waiting for auto-update" and leaves `w.db == nil`.
`startUpdate()` calls `dbsource.Start(w.sourceCfg(), w.logger, onUpdate)` and stores the result in `w.updater`; the callback ignores empty paths and paths equal to `w.sourceDbPath`, else calls `w.hotSwap(path)`.

**`hotSwap`** creates a new local copy, opens it, reads the version, swaps `db`/`path`/`version`/`currentLocalDbCopy`/`sourceDbPath` **without any mutex**, and then closes the old handle on a detached goroutine after a hard `time.Sleep(10 * time.Second)`:
```go
if oldDB != nil {
	go func() { time.Sleep(10 * time.Second); oldDB.Close() }()
}
```
That goroutine is unowned and unjoinable — a second background worker a `Sleep()` would have to reason about.

**`Close()`**
```go
// Close stops the updater and the file handle. The reclaim table calls this when the incarnation ends.
func (w *BIN) Close() { w.close() }

func (w *BIN) close() {
	if w.updater != nil { w.updater.Stop() }
	if w.db != nil { w.db.Close(); w.db = nil }
}
```
Signature is `Close()` with no error, which is exactly what `reclaim.closer` requires. Idempotent-ish (`w.db` nil-checked and nil'd; `Updater.Stop` is nil-safe and close-once) but **not** goroutine-safe and **not** joining anything. After `Close()`, `LookupRecord` returns `fmt.Errorf("BIN is not open")` — which is what the dbwrappers tests use as the "H1 loop must be stopped" probe.

**Other exported surface:** `LookupRecord(ip string, fields FieldMap) (dbprovider.Record, error)`, `Version() *dbutils.DBVersion`, `Path() string`, `SourcePath() string`, plus the `DefaultBINMinAge = 30 * 24 * time.Hour` constant.

## 11.2 `mmdb.go` (183 lines)

```go
type MMDBConfig struct {
	Dir             string
	Source          dbsource.Config
	DefaultFileName string
	MinAge          time.Duration
}

type MMDB struct {
	mu      sync.RWMutex          // guards db and path — BIN has no equivalent
	db      *maxminddb.Reader
	path    string
	logger  *slog.Logger
	cfg     MMDBConfig
	updater *dbsource.Updater
}

const keyPrefixMMDB = "mmdb:"
func mmdbKey(cfg MMDBConfig) string { return keyPrefixMMDB + cfg.Source.Key + ":" + configHash(cfg) }

func OpenMMDB(ctx context.Context, cfg MMDBConfig, logger *slog.Logger) (*MMDB, error) {
	key := mmdbKey(cfg)
	v, err := reclaim.Open(ctx, key, logger, func() (any, error) { return newMMDB(cfg, logger) })
	...
	w, ok := v.(*MMDB)   // else fmt.Errorf("reclaim: %s: want *MMDB, got %T", key, v)
}
```
Only **one** logger here — MMDB has no owner-plugin split.

`newMMDB` resolves the path, calls `w.open(path)`, then unconditionally calls `dbsource.Start(...)` (which returns `(nil, nil)` when there is no URL) and stores `w.updater`. The update callback skips empty paths and `path == w.Path()`, else `w.open(path)` and logs "failed to open updated MMDB" on error.

`open(path)` reads the whole file with `os.ReadFile`, builds `maxminddb.FromBytes`, swaps `db`/`path` under `w.mu.Lock()`, then closes the old reader **outside** the lock and logs `Info "MMDB opened"`. So MMDB's hot-swap is immediate, unlike BIN's 10-second delayed close.

**`Close()`**
```go
func (w *MMDB) Close() { w.close() }

func (w *MMDB) close() {
	if w.updater != nil { w.updater.Stop() }
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.db != nil { _ = w.db.Close(); w.db = nil }
}
```
After `Close()`, `Lookup` returns `fmt.Errorf("MMDB is not open")` because `db == nil`.

Other surface: `Path() string` (RLock), `LookupRecord(ip, fields)`, `Lookup(ip string, dest any) error`, and the shared `configHash(v any) string` helper (JSON + FNV-64a, hex) which lives in this file and is used by both wrappers.

## 11.3 `reset.go` (17 lines) — the whole file

```go
package dbwrappers

import (
	"time"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

// Reset disposes every singleton wrapper. Tests only.
func Reset() { reclaim.Reset() }

// ResetWith is Reset with a grace. Tests only.
func ResetWith(grace time.Duration) { reclaim.ResetWith(grace) }
```
Pure delegation to the process table. Note this is the *only* teardown path for wrappers, and on master it does not wait for `Close()` to return.

## 11.4 `reclaim_test.go` on master (327 lines, 4 tests)

**Helpers** (a slimmer duplicate of the `pkg/reclaim` set — `recHandler` is defined *twice* in the repo, once per package):
- `recHandler` with `Enabled`/`Handle`/`WithAttrs`/`WithGroup`/`events()` — identical structure to the reclaim-package one, `[][2]string` of `{msg, key}`.
- `hasEvent(got [][2]string, msg, key string) bool` — membership.
- `hasSubseq(got, want [][2]string) bool` — ordered subsequence match.
- `useShortLeases(t *testing.T) *recHandler` — `ResetWith(25 * time.Millisecond)` and returns a fresh recorder. **This 25 ms lease window is the hardening target the card mentions.**
- `silentTickerURL(t *testing.T) string` — an `httptest.NewServer` that always 404s, so the updater's ticker has a URL to hit that never yields a file; `t.Cleanup(srv.Close)`.
- Referenced from sibling test files: `testBIN`, `testLogger()`, `testBINRecord(t, w)`, `testLiteMMDB(t)`, `mustFields(t, PresetIP2LocationLite)`.

**The four tests**
1. `TestOpenBIN_SameHashReclaimKeepsTicker` — `Reset()`, `t.Cleanup(Reset)`, short leases; open with `ctx1`, assert `a.updater != nil`; `cancel1()`; open again with `ctx2`; assert `a == b` **and `a.updater == b.updater`** (one ticker); lookup returns `US`; **`time.Sleep(80 * time.Millisecond)`** (past the 25 ms lease) and lookup again still returns `US`. This is defect 2: it never reached the reclaim branch.
2. `TestOpenBIN_HashChangeDisposesOld` — two configs with different `Source.Key`/`Path` (`h1`, `h2`) so the hashes differ; asserts key prefixes `bin:h1:` / `bin:h2:`; opens H1 with the spy logger, cancels, opens H2, sleeps 80 ms; asserts `h1.LookupRecord(...)` now **errors** ("H1 loop must be stopped"), H2 still resolves `US`; asserts the event subsequence `[{MsgPut,key1},{MsgOrphan,key1},{MsgDispose,key1}]` and `hasEvent(MsgPut, key2)` and **not** `hasEvent(MsgDispose, key2)`.
3. `TestOpenMMDB_SameHashReclaimKeepsTicker` — the MMDB mirror of (1), asserting `a == b && a.updater == b.updater` and a `country_code == "US"` lookup before and after the 80 ms sleep.
4. `TestOpenMMDB_HashChangeDisposesOld` — the MMDB mirror of (2), with `mmdb:h1:` / `mmdb:h2:` prefixes.

On fix-reclaim this file grows to 406 lines: the two `SameHashReclaim` tests wait for `reclaim_orphan` and assert the `reclaim_reclaim` line, and a `settleFirstTick` helper is added to work around the `pkg/dbsource` shutdown gap.

## 11.5 What a `Sleep()` / `Wake()` would have to stop and restart

Per **`*BIN`** incarnation:
- **`w.updater`** — the `dbsource.Updater` ticker goroutine. Sleep must `Stop()` it *and* be able to guarantee the in-flight `tick` is finished (today it cannot; see §10.5). Wake must restart it, which means re-running `startUpdate()` (re-creating ticker + stop channel) and accepting the immediate unconditional `tick`.
- **`w.db`** — the `*ip2loc.DB` file handle plus the on-disk temp copy at `w.currentLocalDbCopy`. Sleep must decide whether to close the handle (and whether to delete the temp copy — nothing deletes it today) and Wake must re-`OpenDB` and re-read the version. Note `LookupRecord` fails with "BIN is not open" whenever `w.db == nil`, so a sleeping BIN cannot serve lookups unless Sleep keeps the handle.
- **The delayed hot-swap closer** — `hotSwap` spawns `go func(){ time.Sleep(10*time.Second); oldDB.Close() }()`, an unowned goroutine holding a stale handle. Sleep has no way to cancel or join it.
- **State that must survive a sleep/wake pair:** `cfg`, `logger`, `path`, `version`, `sourceDbPath`, `currentLocalDbCopy`.

Per **`*MMDB`** incarnation:
- **`w.updater`** — same story as BIN.
- **`w.db`** — the `*maxminddb.Reader` (whole file already read into memory). Sleep would close it under `w.mu`; Wake would re-`open(w.path)`, which re-reads the file from disk. `Lookup` returns "MMDB is not open" while `db == nil`.
- MMDB at least already has `w.mu sync.RWMutex` guarding `db`/`path`, so a Sleep/Wake there has a lock to use; **`BIN` has no mutex at all**, so adding Sleep/Wake to `*BIN` means adding synchronization that does not exist today (`hotSwap` already mutates `w.db` unguarded).

Per **`*geoblock.Plugin`** incarnation (the third reclaim call site, `plugin.go:44`):
- `bindPlugin` does `reclaim.Open(ctx, pluginKey(name, cfg), geoblock.PluginLogger(name, cfg), func() (any, error) { return geoblock.NewCore(name, cfg) })` and then `pluginInstance.ForRoute(next)`. Key is `"plugin:" + name + ":" + pluginConfigHash(cfg)` where the hash is JSON+FNV of the *prepared* config. Whatever `*geoblock.Plugin` owns transitively (it holds the wrappers) would need the same Sleep/Wake treatment; it is not read in this survey.

Cross-cutting: the table itself calls only `Close()` (via the unexported `closer` interface). A Sleep/Wake lifecycle means either a second optional interface asserted the same way (e.g. `interface{ Sleep(); Wake() }`) or a change to `closer` — and under the shared-copy no-removal rule, adding a second optional interface is the additive shape, while reshaping `closer` is not.

---

# 12. Do the spec and devdocs exist on `origin/master`?

**Both exist.** `openspec/specs/std_go_reclaim_context-lease/spec.md` (109 lines) and `knowledge/devdocs/std_go_reclaim.md` (57 lines).

## 12.1 Master spec — requirement headers and scenario names

```
### Requirement: Table file depends only on the Go standard library
#### Scenario: Stdlib-only imports

### Requirement: Process table is a singleton
#### Scenario: Default Open shares one incarnation

### Requirement: Open creates once and binds a context
#### Scenario: Two holders one dispose
#### Scenario: Second create dispose is ignored
#### Scenario: Lost create race
#### Scenario: Missing context panics
#### Scenario: Nil logger is rejected

### Requirement: Cancel then open within grace does not dispose
#### Scenario: Reclaim before grace
#### Scenario: Grace elapses without rebind

### Requirement: Keys are independent
#### Scenario: One key times out

### Requirement: Grace is configurable
#### Scenario: Default grace
#### Scenario: Zero grace

### Requirement: Lifecycle events are logged
#### Scenario: Hash change orphan then dispose
#### Scenario: Reset logs dispose
#### Scenario: Open logger level gates put and dispose
```

Six requirements, 14 scenarios (vs eight requirements and 22 scenarios on fix-reclaim).

**What master is missing relative to fix-reclaim:** the whole `Requirement: Incarnation end closes the stored value before it reports the end` block (3 scenarios), the whole `Requirement: Either side of the grace edge is correct` block (1 scenario), the three per-key-grace scenarios, `Scenario: Orphan precedes dispose at a short grace`, and `Scenario: Orphan and dispose use the last binding logger`.

**Master's `Grace is configurable` body**, verbatim — note it is table-scoped:
> The table SHALL use a caller-supplied grace duration. A zero grace SHALL cancel the lifetime as soon as the last holder is gone (no wait). A negative grace SHALL become the product default of 10 seconds. Default grace in this product SHALL be 10 seconds (`DefaultGrace`) when the process table is constructed.

**Master's `Lifecycle events are logged` body** ends at *"Log lines MUST NOT be emitted while the table mutex is held."* — it contains **no** orphan-before-dispose sentence and **no** exception clauses. Master's `Open creates once and binds a context` header signature is `Open(ctx, key, logger, create)` with no `grace` clause.

## 12.2 Master devdocs — terms and gotchas

Master defines **five** terms (fix-reclaim adds **Incarnation** as a sixth): **Table**, **Default**, **Open**, **Grace**, **Lifetime**. Differences that matter:

- **Open** on master has no `grace` argument in its description and no "the table waits before logging dispose" clause.
- **Grace** on master is one sentence plus "Negative grace is `DefaultGrace` (10s)"; *Avoid:* "passing `0` when you meant the product default". No per-incarnation ownership, no `TableGrace`.
- **Lifetime** on master is described as *"The context `create` receives"* — which is stale even on master, since `create` takes no arguments. fix-reclaim rewrote it as "the per-incarnation context the table holds… `create` takes no arguments, so callers never see this context."
- Master's Overview has **no** "This package is a shared copy" paragraph and **no** bidirectional-sync/no-removal rule. That rule was written during this ticket.
- Master's pattern snippet is the 4-arg call: `reclaim.Open(ctx, "bin:"+hash, logger, func() (any, error) {...})`.

**Master's Gotchas section — verbatim, all four bullets:**

> ## Gotchas
>
> - Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`). `NewTable(0)` ends the incarnation as soon as the last holder is gone.
> - Yaegi: do not write `Table[*T]` on a type from another package.
> - A second `Open` while the incarnation is live or in grace does not replace the lifetime.
> - Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.

The four bullets fix-reclaim adds are the `Close()`-flag-is-not-a-signal rule, the "cancel-then-Open is usually not a reclaim" trap, the "a test must never need a specific timer to win" rule, and the orphan-before-arm ordering note with its two stated exceptions.

---

# Appendix: things a redesign should decide early

1. **The no-removal rule is a human decision on record**, not a preference. Deleting the per-slot lifetime context, its goroutine, or `arming` reverses "Should the lifetime context and its per-slot goroutine be deleted, since closing inline is simpler? — resolved by the human: no." It also breaks the file-copy port to `traefik-modsecurity`, where `default.go` is byte-identical and `table.go` differs only in local variable names.
2. **`Open` returns `(any, error)`, not a handle.** Release is a `watch` goroutine per `Open` call. Removing goroutines means inventing a release mechanism, which is an API change on a shared copy.
3. **The `Close()`-then-log ordering is now a spec requirement with three scenarios and two dedicated tests.** Any new lifecycle has to keep it or renegotiate the spec.
4. **`context.Background()` holders are polled every 20 ms forever.** Both the fix-reclaim card ("Rank-up moves") and `TestTable_HolderWithoutDoneChannelIsPolled` treat this as live behaviour, because Yaegi hands the plugin a context whose `Done()` is nil.
5. **`pkg/dbsource.Updater` is the real blocker for a quiesce-style `Sleep()`.** It cannot be joined and cannot cancel an in-flight download. That gap already has an open follow-up on the fix-reclaim card, and `pkg/dbwrappers/reclaim_test.go` carries an 80 ms sleep (later a `settleFirstTick` helper) purely to work around it.
6. **`*BIN` has no mutex**, and `hotSwap` mutates `w.db` unguarded plus spawns an unowned 10-second delayed closer goroutine. `*MMDB` at least has `sync.RWMutex`.
