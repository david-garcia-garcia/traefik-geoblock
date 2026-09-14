package reclaim

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// waitBudget guards a condition that should already be true. It is not a timing assertion.
const waitBudget = 10 * time.Second

// unstuckValue is what a later Open creates after a panicking hook unstuck the key.
const unstuckValue = "next"

// firstIncarnation and nextIncarnation are the stored values in Close-vs-create overlap tests.
const firstIncarnation = "first"
const nextIncarnation = "second"

// graceNoRace is long enough that a test asserting the reclaim branch cannot lose the grace race.
const graceNoRace = 5 * time.Second

// NewTable builds a table for tests. Production callers use New(Config).
func NewTable(grace time.Duration) *Table {
	return New(Config{Grace: grace})
}

// box is a disposable stand-in stored on the table in tests.
type box struct {
	n     int
	ended *atomic.Bool
}

// Close marks ended when the table stops this incarnation.
func (b *box) Close() {
	if b.ended != nil {
		b.ended.Store(true)
	}
}

// ending is a box that sets done when Close runs.
func ending(n int, done *atomic.Bool) *box {
	return &box{n: n, ended: done}
}

// namedEnd appends name to ended when the table stops that incarnation.
type namedEnd struct {
	name  string
	mu    *sync.Mutex
	ended *[]string
}

// Close records this name as stopped.
func (n *namedEnd) Close() {
	n.mu.Lock()
	*n.ended = append(*n.ended, n.name)
	n.mu.Unlock()
}

// counterClose counts how many times the table closed this value.
type counterClose struct {
	closes atomic.Int32
}

// Close counts one disposal of this value.
func (c *counterClose) Close() { c.closes.Add(1) }

// lifecycle records every event the table drives on one stored value, in order. It is the
// instrument for the create / sleep / wake / close contract.
type lifecycle struct {
	mu     sync.Mutex
	events []string
	// onSleep runs inside Sleep, so a test can hold an Open inside the sleep transition. That is
	// the only way to reach the sleeping window deterministically: an Open that merely races a
	// cancel almost always arrives while the holder is still counted, and is a plain second bind.
	onSleep func()
	// onWake runs inside Wake, so a test can hold an Open inside the wake transition.
	onWake func()
	// onClose runs inside Close, so a test can see what the table had logged before it returned.
	onClose func()
}

// Sleep records that the table put this value to sleep, after running any test hook.
func (l *lifecycle) Sleep() {
	if l.onSleep != nil {
		l.onSleep()
	}
	l.record("sleep")
}

// Wake records that the table woke this value, after running any test hook.
func (l *lifecycle) Wake() {
	if l.onWake != nil {
		l.onWake()
	}
	l.record("wake")
}

// Close records that the table closed this value, then runs any test hook.
func (l *lifecycle) Close() {
	l.record("close")
	if l.onClose != nil {
		l.onClose()
	}
}

// record appends one lifecycle event.
func (l *lifecycle) record(event string) {
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
}

// seq is the events this value received, in order.
func (l *lifecycle) seq() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

// count is how many times this value received one event.
func (l *lifecycle) count(event string) int {
	n := 0
	for _, got := range l.seq() {
		if got == event {
			n++
		}
	}
	return n
}

// onlyCloser implements Close but neither Sleep nor Wake, like a value written before this
// lifecycle existed.
type onlyCloser struct {
	closes atomic.Int32
}

// Close counts one disposal.
func (o *onlyCloser) Close() { o.closes.Add(1) }

// recHandler records slog lines so tests can assert reclaim msg + key + level.
type recHandler struct {
	mu   sync.Mutex
	recs []slog.Record
}

// Enabled keeps every level so debug reclaim lines are captured.
func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }

// Handle stores a clone of the record.
func (h *recHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

// WithAttrs returns the same handler; tests do not use slog attributes on the handler itself.
func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup returns the same handler; tests do not use slog groups.
func (h *recHandler) WithGroup(string) slog.Handler { return h }

// events is msg + key for each recorded line, in order.
func (h *recHandler) events() [][2]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([][2]string, 0, len(h.recs))
	for _, r := range h.recs {
		var key string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "key" {
				key = a.Value.String()
			}
			return true
		})
		out = append(out, [2]string{r.Message, key})
	}
	return out
}

// hookPanicLine is one recorded reclaim_hook_panic: its level and the attrs a reader needs.
type hookPanicLine struct {
	level slog.Level
	key   string
	hook  string
	panic string
}

// hookPanics is every recorded reclaim_hook_panic line, in order.
func (h *recHandler) hookPanics() []hookPanicLine {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]hookPanicLine, 0, len(h.recs))
	for _, r := range h.recs {
		if r.Message != MsgHookPanic {
			continue
		}
		line := hookPanicLine{level: r.Level}
		r.Attrs(func(a slog.Attr) bool {
			switch a.Key {
			case "key":
				line.key = a.Value.String()
			case "hook":
				line.hook = a.Value.String()
			case "panic":
				line.panic = a.Value.String()
			}
			return true
		})
		out = append(out, line)
	}
	return out
}

// requireLevels fails if any of the five reclaim messages was not logged at debug.
func (h *recHandler) requireLevels(t *testing.T) {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.recs {
		switch r.Message {
		case MsgPut, MsgBind, MsgOrphan, MsgReclaim, MsgDispose:
			if r.Level != slog.LevelDebug {
				t.Fatalf("%s logged at %v, want debug", r.Message, r.Level)
			}
		}
	}
}

// levelGate drops lines below min, so a test can prove the Open logger gates the reclaim lines.
type levelGate struct {
	min slog.Level
	recHandler
}

// Enabled reports whether l passes this gate.
func (h *levelGate) Enabled(_ context.Context, l slog.Level) bool { return l >= h.min }

// keySeq is the message sequence for one key.
func keySeq(ev [][2]string, key string) []string {
	var out []string
	for _, e := range ev {
		if e[1] == key {
			out = append(out, e[0])
		}
	}
	return out
}

// countKeyMsg counts one message for one key.
func countKeyMsg(ev [][2]string, msg, key string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg && e[1] == key {
			n++
		}
	}
	return n
}

// countMsg counts one message across every key.
func countMsg(ev [][2]string, msg string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg {
			n++
		}
	}
	return n
}

// waitUntil fails if cond is still false after waitBudget.
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitBudget)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout waiting for condition")
}

// openReturned is Open that must come back. A leftover busy slot hangs until waitBudget, then the test fails.
func openReturned(ctx context.Context, t *testing.T, tab *Table, key string, logger *slog.Logger, create func() (any, error), hooks Hooks) (any, error) {
	t.Helper()
	type finished struct {
		value any
		err   error
	}
	done := make(chan finished, 1)
	go func() {
		value, err := tab.Open(ctx, key, logger, create, hooks)
		done <- finished{value, err}
	}()
	select {
	case got := <-done:
		return got.value, got.err
	case <-time.After(waitBudget):
		t.Fatal("Open did not return; key left busy")
		return nil, nil
	}
}

// waitKeyMsg waits until msg is logged for key.
func waitKeyMsg(t *testing.T, h *recHandler, msg, key string) {
	t.Helper()
	waitUntil(t, func() bool { return countKeyMsg(h.events(), msg, key) > 0 })
}

// mustSlot returns the mapped incarnation for key or fails.
func mustSlot(t *testing.T, tab *Table, key string) *slot {
	t.Helper()
	tab.mu.Lock()
	defer tab.mu.Unlock()
	incarnation := tab.items[key]
	if incarnation == nil {
		t.Fatalf("missing slot %s", key)
	}
	return incarnation
}

// readState reads one incarnation's state under the table mutex.
func readState(tab *Table, incarnation *slot) slotState {
	tab.mu.Lock()
	defer tab.mu.Unlock()
	return incarnation.state
}

// holderCount reads one incarnation's holder count under the table mutex.
func holderCount(tab *Table, incarnation *slot) int {
	tab.mu.Lock()
	defer tab.mu.Unlock()
	return incarnation.holders
}

// mappedKeys is how many keys the table still stores.
func mappedKeys(tab *Table) int {
	tab.mu.Lock()
	defer tab.mu.Unlock()
	return len(tab.items)
}

// settledGoroutines is a goroutine count after the scheduler has had a moment.
func settledGoroutines() int {
	time.Sleep(20 * time.Millisecond)
	return runtime.NumGoroutine()
}

// nilDoneCtx is the Yaegi holder shape: Done() is nil, so the table must poll Err().
type nilDoneCtx struct {
	canceled atomic.Bool
}

// Deadline reports no deadline.
func (c *nilDoneCtx) Deadline() (time.Time, bool) { return time.Time{}, false }

// Done returns nil, which is what makes this context need polling.
func (c *nilDoneCtx) Done() <-chan struct{} { return nil }

// Err reports cancellation once cancel has been called.
func (c *nilDoneCtx) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	return nil
}

// Value has nothing to look up.
func (c *nilDoneCtx) Value(any) any { return nil }

// cancel ends this holder.
func (c *nilDoneCtx) cancel() { c.canceled.Store(true) }

// recLogger is a debug logger writing into h.
func recLogger(h slog.Handler) *slog.Logger { return slog.New(h) }

// testLifeHooks binds life's Sleep, Wake, and Close methods as Open hooks.
func testLifeHooks(life *lifecycle) Hooks {
	return Hooks{Sleep: life.Sleep, Wake: life.Wake, Close: life.Close}
}

// testCloseHook is a Close-only Open hook for tests that assert disposal of a value.
func testCloseHook(onClose func()) Hooks {
	return Hooks{Close: onClose}
}

func TestTable_OpenCancelDispose(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 30 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	var ended atomic.Bool
	stored := ending(1, &ended)
	value, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return stored, nil }, testCloseHook(stored.Close))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if value.(*box).n != 1 {
		t.Fatalf("wrong value: %#v", value)
	}

	cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
	if !ended.Load() {
		t.Fatal("dispose logged before Close returned")
	}
	if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("sequence %v", got)
	}
	h.requireLevels(t)
}

func TestTable_LifecycleIsCreateSleepWakeClose(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}
	created := 0

	for cycle := 0; cycle < 5; cycle++ {
		ctx, cancel := context.WithCancel(context.Background())
		got, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
			created++
			return life, nil
		}, testLifeHooks(life))
		if err != nil {
			t.Fatalf("open %d: %v", cycle, err)
		}
		if got != any(life) {
			t.Fatalf("cycle %d returned a different value", cycle)
		}
		cancel()
		waitUntil(t, func() bool { return life.count("sleep") == cycle+1 })
	}

	if created != 1 {
		t.Fatalf("create ran %d times, want 1", created)
	}
	// Four wakes, because the first Open created the value awake rather than waking it.
	want := []string{
		"sleep", "wake", "sleep", "wake", "sleep", "wake", "sleep", "wake", "sleep",
	}
	if got := life.seq(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle %v, want %v", got, want)
	}
	if got := countKeyMsg(h.events(), MsgReclaim, "a"); got != 4 {
		t.Fatalf("%d reclaim lines, want 4", got)
	}

	tab.Reset()
	waitKeyMsg(t, h, MsgDispose, "a")
	if got := life.count("close"); got != 1 {
		t.Fatalf("close ran %d times, want 1", got)
	}
	if got := life.count("sleep"); got != 5 {
		t.Fatalf("sleep ran %d times after Reset, want 5 (the already sleeping value is not slept twice)", got)
	}
}

func TestTable_LaterOpenDoesNotReplaceHooks(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}
	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel1()
	waitKeyMsg(t, h, MsgOrphan, "a")

	ignored := &lifecycle{}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if _, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) { t.Error("create ran again"); return nil, nil }, testLifeHooks(ignored)); err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if got := life.count("wake"); got != 1 {
		t.Fatalf("stored Wake ran %d times, want 1", got)
	}
	if got := ignored.count("wake"); got != 0 {
		t.Fatalf("later Open's Wake ran %d times, want 0", got)
	}

	cancel2()
	waitUntil(t, func() bool { return life.count("sleep") == 2 })
	tab.Reset()
	waitKeyMsg(t, h, MsgDispose, "a")
	if got := life.count("close"); got != 1 {
		t.Fatalf("stored Close ran %d times, want 1", got)
	}
	if got := ignored.count("close"); got != 0 {
		t.Fatalf("later Open's Close ran %d times, want 0", got)
	}
}

func TestTable_SleepPrecedesCloseAtEveryGrace(t *testing.T) {
	for _, grace := range []time.Duration{0, 5 * time.Millisecond} {
		t.Run(grace.String(), func(t *testing.T) {
			h := &recHandler{}
			tab := New(Config{Grace: grace})
			life := &lifecycle{}
			ctx, cancel := context.WithCancel(context.Background())
			if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
				t.Fatalf("open: %v", err)
			}
			cancel()
			waitKeyMsg(t, h, MsgDispose, "a")
			if got := life.seq(); !reflect.DeepEqual(got, []string{"sleep", "close"}) {
				t.Fatalf("lifecycle %v, want sleep then close", got)
			}
		})
	}
}

func TestTable_OpenWaitsForWake(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	release := make(chan struct{})
	life := &lifecycle{onWake: func() { <-release }}

	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel1()
	waitKeyMsg(t, h, MsgOrphan, "a")

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	returned := make(chan any, 1)
	go func() {
		got, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) { t.Error("create ran again"); return nil, nil }, Hooks{})
		if err != nil {
			t.Errorf("open 2: %v", err)
		}
		returned <- got
	}()

	// Wake is blocked, so Open must not have returned and must not have logged its bind.
	time.Sleep(50 * time.Millisecond)
	select {
	case <-returned:
		t.Fatal("Open returned a value while Wake was still running")
	default:
	}
	if got := countKeyMsg(h.events(), MsgBind, "a"); got != 1 {
		t.Fatalf("%d bind lines while Wake was blocked, want 1 (the first Open's)", got)
	}

	close(release)
	got := <-returned
	if got != any(life) {
		t.Fatal("Open returned a different value")
	}
	if life.count("wake") != 1 {
		t.Fatalf("wake ran %d times, want 1", life.count("wake"))
	}
}

func TestTable_ConcurrentOpensOnSleepingValueWakeOnce(t *testing.T) {
	const openers = 8
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}

	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel1()
	waitKeyMsg(t, h, MsgOrphan, "a")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < openers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { t.Error("create ran again"); return nil, nil }, Hooks{})
			if err != nil {
				t.Errorf("open: %v", err)
			}
			if got != any(life) {
				t.Error("open returned a different value")
			}
		}()
	}
	wg.Wait()

	if got := life.count("wake"); got != 1 {
		t.Fatalf("wake ran %d times, want 1", got)
	}
	if got := countKeyMsg(h.events(), MsgReclaim, "a"); got != 1 {
		t.Fatalf("%d reclaim lines, want 1", got)
	}
	if got := countKeyMsg(h.events(), MsgBind, "a"); got != openers+1 {
		t.Fatalf("%d bind lines, want %d", got, openers+1)
	}
}

func TestTable_ConcurrentFirstOpensCreateOnce(t *testing.T) {
	const openers = 8
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	var created atomic.Int32
	var values []*counterClose
	var valuesMu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make([]any, openers)
	var wg sync.WaitGroup
	for i := 0; i < openers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
				created.Add(1)
				// A GeoIP download is not instant; hold the slot so the others must wait.
				time.Sleep(10 * time.Millisecond)
				made := &counterClose{}
				valuesMu.Lock()
				values = append(values, made)
				valuesMu.Unlock()
				return made, nil
			}, Hooks{})
			if err != nil {
				t.Errorf("open %d: %v", i, err)
			}
			got[i] = value
		}(i)
	}
	wg.Wait()

	if n := created.Load(); n != 1 {
		t.Fatalf("create ran %d times, want 1", n)
	}
	for i := range got {
		if got[i] != got[0] {
			t.Fatalf("open %d returned a different value", i)
		}
	}
	// No value may be created and then thrown away: the one create is the one stored.
	valuesMu.Lock()
	defer valuesMu.Unlock()
	if len(values) != 1 {
		t.Fatalf("%d values created, want 1", len(values))
	}
	if n := values[0].closes.Load(); n != 0 {
		t.Fatalf("the stored value was closed %d times while it is still held", n)
	}
	if got := countKeyMsg(h.events(), MsgPut, "a"); got != 1 {
		t.Fatalf("%d put lines, want 1", got)
	}
}

func TestTable_CreateErrorReachesEveryWaiter(t *testing.T) {
	const openers = 4
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	boom := errors.New("create failed")
	var created atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make([]error, openers)
	var wg sync.WaitGroup
	for i := 0; i < openers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
				// Only the first create fails. A caller that ran its own create would get a
				// value instead of the error, which is what this test is looking for.
				if created.Add(1) == 1 {
					time.Sleep(10 * time.Millisecond)
					return nil, boom
				}
				return &counterClose{}, nil
			}, Hooks{})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if !errors.Is(err, boom) {
			t.Fatalf("open %d returned %v, want the first create's error", i, err)
		}
	}
	if got := countKeyMsg(h.events(), MsgPut, "a"); got != 0 {
		t.Fatalf("%d put lines after a failed create, want 0", got)
	}
	if mappedKeys(tab) != 0 {
		t.Fatal("a failed create left the key stored")
	}

	// A later Open is free to try again.
	value, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return &counterClose{}, nil }, Hooks{})
	if err != nil || value == nil {
		t.Fatalf("retry after a failed create: value %v err %v", value, err)
	}
}

func TestTable_OpenDuringGraceReclaims(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	first, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return ending(1, &ended), nil }, Hooks{})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel1()
	// Wait for the orphan line: an Open right after cancel is usually a plain second bind.
	waitKeyMsg(t, h, MsgOrphan, "a")

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) { t.Error("create ran again"); return nil, nil }, Hooks{})
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if first != second {
		t.Fatal("reclaim returned a different value")
	}
	if ended.Load() {
		t.Fatal("the reclaimed incarnation was closed")
	}
	if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgReclaim, MsgBind}) {
		t.Fatalf("sequence %v", got)
	}
	if state := readState(tab, mustSlot(t, tab, "a")); state != slotAwake {
		t.Fatalf("slot state %v after reclaim, want awake", state)
	}
	h.requireLevels(t)
}

func TestTable_SecondCreateDisposeIgnored(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	created := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	make1 := func() (any, error) { created++; return ending(1, nil), nil }
	first, err := tab.Open(ctx, "a", recLogger(h), make1, Hooks{})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, err := tab.Open(ctx, "a", recLogger(h), make1, Hooks{})
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if created != 1 {
		t.Fatalf("create ran %d times, want 1", created)
	}
	if first != second {
		t.Fatal("second Open returned a different value")
	}
}

func TestTable_TwoOpensOneDispose(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 10 * time.Millisecond})
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return ending(1, &ended), nil }, Hooks{}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) { t.Error("create ran again"); return nil, nil }, Hooks{}); err != nil {
		t.Fatalf("open 2: %v", err)
	}

	cancel1()
	time.Sleep(60 * time.Millisecond)
	if ended.Load() {
		t.Fatal("closed while a live holder remained")
	}

	cancel2()
	waitKeyMsg(t, h, MsgDispose, "a")
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatal("more than one dispose")
	}
}

func TestTable_NegativeGraceUsesDefault(t *testing.T) {
	if got := New(Config{Grace: -1}).grace; got != DefaultGrace {
		t.Fatalf("grace %v, want %v", got, DefaultGrace)
	}
}

func TestTable_ZeroGraceEndsImmediately(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	var ended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	stored := ending(1, &ended)
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return stored, nil }, testCloseHook(stored.Close)); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
	if !ended.Load() {
		t.Fatal("dispose logged before Close returned")
	}
	if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("sequence %v", got)
	}
	if mappedKeys(tab) != 0 {
		t.Fatal("zero grace kept the key stored")
	}
	h.requireLevels(t)
}

func TestTable_ZeroGraceRacingOpenIsPlainBind(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	// Hold the first value inside Sleep, so the second Open is guaranteed to arrive in the
	// window a zero-grace table must not keep. Racing two goroutines instead would let the Open
	// win and be a plain second bind, which is not the case under test.
	sleeping := make(chan struct{})
	release := make(chan struct{})
	first := &lifecycle{onSleep: func() { close(sleeping); <-release }}

	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return first, nil }, testLifeHooks(first)); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel1()
	<-sleeping

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second := &lifecycle{}
	opened := make(chan any, 1)
	go func() {
		value, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) { return second, nil }, testLifeHooks(second))
		if err != nil {
			t.Errorf("open 2: %v", err)
		}
		opened <- value
	}()

	// The second Open is parked on the sleeping slot and cannot be served yet.
	time.Sleep(50 * time.Millisecond)
	select {
	case <-opened:
		t.Fatal("Open was served from a value that was still being slept")
	default:
	}

	close(release)
	got := <-opened
	if got != any(second) {
		t.Fatal("zero grace kept the sleeping value and handed it back")
	}
	if first.count("wake") != 0 {
		t.Fatal("a zero-grace table woke a value it was supposed to drop")
	}
	// A zero-grace table has no grace window, so nothing can be reclaimed into one.
	if n := countMsg(h.events(), MsgReclaim); n != 0 {
		t.Fatalf("%d reclaim lines at zero grace, want 0 (sequence %v)", n, keySeq(h.events(), "a"))
	}
	waitKeyMsg(t, h, MsgDispose, "a")
	// Default unmaps before Close, so dispose need not precede the second put.
	if n := countMsg(h.events(), MsgPut); n != 2 {
		t.Fatalf("%d put lines, want 2 (sequence %v)", n, keySeq(h.events(), "a"))
	}
}

// TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns fails if a later Open starts create
// while the previous incarnation's Close is still blocked.
func TestTable_ZeroGraceCreateWaitsUntilPreviousCloseReturns(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-releaseClose:
		default:
			close(releaseClose)
		}
	})
	var createWhileCloseBlocked atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) { return firstIncarnation, nil }, Hooks{
		Close:                  func() { close(closeEntered); <-releaseClose },
		EnforceCloseBeforeOpen: true,
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	// Hold Close so a racing Open cannot create until this incarnation has ended.
	<-closeEntered

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan error, 1)
	go func() {
		_, err := tab.Open(ctx2, "k", recLogger(h), func() (any, error) {
			createWhileCloseBlocked.Store(true)
			return nextIncarnation, nil
		}, Hooks{})
		opened <- err
	}()
	// The second Open must stay parked for the whole Close, not create during it.
	select {
	case err := <-opened:
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
		t.Fatalf("second Open returned before Close returned: %v", err)
	case <-time.After(200 * time.Millisecond):
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
	}
	// Release Close; the parked Open may create only after that returns.
	close(releaseClose)
	select {
	case err := <-opened:
		if err != nil {
			t.Fatalf("open 2: %v", err)
		}
	case <-time.After(waitBudget):
		t.Fatal("second Open did not return after Close returned")
	}
	if !createWhileCloseBlocked.Load() {
		t.Fatal("second Open did not create after Close returned")
	}
}

// TestTable_ExpireCreateWaitsUntilPreviousCloseReturns is the same overlap after grace elapsed.
func TestTable_ExpireCreateWaitsUntilPreviousCloseReturns(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: time.Millisecond})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-releaseClose:
		default:
			close(releaseClose)
		}
	})
	var createWhileCloseBlocked atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) { return firstIncarnation, nil }, Hooks{
		Close:                  func() { close(closeEntered); <-releaseClose },
		EnforceCloseBeforeOpen: true,
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	// Wait until expire has entered Close (grace elapsed), then hold it.
	select {
	case <-closeEntered:
	case <-time.After(waitBudget):
		t.Fatal("Close did not run after grace elapsed")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan error, 1)
	go func() {
		_, err := tab.Open(ctx2, "k", recLogger(h), func() (any, error) {
			createWhileCloseBlocked.Store(true)
			return nextIncarnation, nil
		}, Hooks{})
		opened <- err
	}()
	// The second Open must stay parked for the whole Close, not create during it.
	select {
	case err := <-opened:
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
		t.Fatalf("second Open returned before Close returned: %v", err)
	case <-time.After(200 * time.Millisecond):
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
	}
	// Release Close; the parked Open may create only after that returns.
	close(releaseClose)
	select {
	case err := <-opened:
		if err != nil {
			t.Fatalf("open 2: %v", err)
		}
	case <-time.After(waitBudget):
		t.Fatal("second Open did not return after Close returned")
	}
	if !createWhileCloseBlocked.Load() {
		t.Fatal("second Open did not create after Close returned")
	}
}

// TestTable_ZeroGraceCreateDoesNotWaitForClose locks the default: with the flag unset, Open
// returns a fresh value while Close is still blocked. That is dest behaviour, not a race assert.
func TestTable_ZeroGraceCreateDoesNotWaitForClose(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-releaseClose:
		default:
			close(releaseClose)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) { return firstIncarnation, nil }, Hooks{
		Close: func() { close(closeEntered); <-releaseClose },
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	<-closeEntered

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan any, 1)
	go func() {
		value, err := tab.Open(ctx2, "k", recLogger(h), func() (any, error) {
			return nextIncarnation, nil
		}, Hooks{})
		if err != nil {
			t.Errorf("open 2: %v", err)
			opened <- nil
			return
		}
		opened <- value
	}()
	select {
	case got := <-opened:
		if got != nextIncarnation {
			t.Fatalf("Open value %v, want a fresh incarnation while Close is blocked", got)
		}
	case <-time.After(waitBudget):
		t.Fatal("Open blocked for Close; default must unmap first")
	}
	close(releaseClose)
}

func TestTable_ResetDuringSleepStillOrphansBeforeDispose(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	sleeping := make(chan struct{})
	release := make(chan struct{})
	life := &lifecycle{onSleep: func() { close(sleeping); <-release }}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	<-sleeping

	// Reset lands while the value is mid-sleep. It must leave this incarnation to the goroutine
	// that owns the transition, or it writes reclaim_dispose before that goroutine's orphan line.
	done := make(chan struct{})
	go func() { tab.Reset(); close(done) }()
	time.Sleep(20 * time.Millisecond)
	close(release)
	<-done

	waitKeyMsg(t, h, MsgDispose, "a")
	want := []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}
	if seq := keySeq(h.events(), "a"); !reflect.DeepEqual(seq, want) {
		t.Fatalf("sequence %v, want %v", seq, want)
	}
	if got := life.count("close"); got != 1 {
		t.Fatalf("close ran %d times, want 1", got)
	}
	if got := life.count("sleep"); got != 1 {
		t.Fatalf("sleep ran %d times, want 1", got)
	}
	if mappedKeys(tab) != 0 {
		t.Fatal("Reset left a key stored")
	}
}

func TestTable_AnOpenThatOnlyWaitsDoesNotTakeTheLogger(t *testing.T) {
	held := &recHandler{}
	waiting := &recHandler{}
	tab := New(Config{Grace: 0})
	sleeping := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	life := &lifecycle{onSleep: func() { once.Do(func() { close(sleeping); <-release }) }}

	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(held), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel1()
	<-sleeping

	// This Open parks on the sleeping incarnation and then creates its own. It never binds to
	// the first one, so the first one's orphan and dispose must not be routed to its logger.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan struct{})
	go func() {
		defer close(opened)
		if _, err := tab.Open(ctx2, "a", recLogger(waiting), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
			t.Errorf("open 2: %v", err)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	close(release)
	<-opened

	waitKeyMsg(t, held, MsgDispose, "a")
	if got := countKeyMsg(waiting.events(), MsgDispose, "a"); got != 0 {
		t.Fatalf("%d dispose lines on the waiting Open's logger, want 0", got)
	}
	if got := countKeyMsg(waiting.events(), MsgOrphan, "a"); got != 0 {
		t.Fatalf("%d orphan lines on the waiting Open's logger, want 0", got)
	}
	want := []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}
	if seq := keySeq(held.events(), "a"); !reflect.DeepEqual(seq, want) {
		t.Fatalf("holder logger sequence %v, want %v", seq, want)
	}
}

func TestTable_ResetRacingADropKeepsOrphanBeforeDispose(t *testing.T) {
	const rounds = 400
	for round := 0; round < rounds; round++ {
		h := &recHandler{}
		tab := New(Config{Grace: graceNoRace})
		ctx, cancel := context.WithCancel(context.Background())
		life := &lifecycle{}
		if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
			t.Fatalf("round %d open: %v", round, err)
		}

		// Reset and the last holder's drop both want to end this incarnation. Whichever writes
		// reclaim_dispose, this incarnation's reclaim_orphan must already be there.
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); cancel() }()
		go func() { defer wg.Done(); tab.Reset() }()
		wg.Wait()

		waitKeyMsg(t, h, MsgDispose, "a")
		seq := keySeq(h.events(), "a")
		want := []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}
		if !reflect.DeepEqual(seq, want) {
			t.Fatalf("round %d: sequence %v, want %v", round, seq, want)
		}
		if got := life.count("close"); got != 1 {
			t.Fatalf("round %d: close ran %d times, want 1", round, got)
		}
		if got := life.count("sleep"); got != 1 {
			t.Fatalf("round %d: sleep ran %d times, want 1", round, got)
		}
	}
}

func TestTable_OpenWaitsForSleepThenWakes(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	sleeping := make(chan struct{})
	release := make(chan struct{})
	// Only the first sleep is held: this value is slept again when the test's own holder goes.
	var once sync.Once
	life := &lifecycle{onSleep: func() { once.Do(func() { close(sleeping); <-release }) }}

	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel1()
	<-sleeping

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan any, 1)
	go func() {
		value, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) { t.Error("create ran again"); return nil, nil }, Hooks{})
		if err != nil {
			t.Errorf("open 2: %v", err)
		}
		opened <- value
	}()

	// A caller must never receive a value that is mid-sleep, so this Open waits.
	time.Sleep(50 * time.Millisecond)
	select {
	case <-opened:
		t.Fatal("Open returned while Sleep was still running")
	default:
	}

	close(release)
	if got := <-opened; got != any(life) {
		t.Fatal("Open returned a different value")
	}
	if seq := life.seq(); !reflect.DeepEqual(seq, []string{"sleep", "wake"}) {
		t.Fatalf("lifecycle %v, want the sleep to finish and the value to be woken", seq)
	}
	want := []string{MsgPut, MsgBind, MsgOrphan, MsgReclaim, MsgBind}
	if seq := keySeq(h.events(), "a"); !reflect.DeepEqual(seq, want) {
		t.Fatalf("sequence %v, want %v", seq, want)
	}
}

func TestTable_OrphanPrecedesDisposeAtTinyGrace(t *testing.T) {
	const rounds = 200
	for round := 0; round < rounds; round++ {
		h := &recHandler{}
		tab := New(Config{Grace: time.Nanosecond})
		ctx, cancel := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
			t.Fatalf("open: %v", err)
		}
		cancel()
		waitKeyMsg(t, h, MsgDispose, "a")
		want := []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}
		if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: sequence %v, want %v", round, got, want)
		}
	}
}

func TestTable_DisposeLogFollowsClose(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 5 * time.Millisecond})
	var sawDispose atomic.Bool
	life := &lifecycle{}
	life.onClose = func() {
		sawDispose.Store(countKeyMsg(h.events(), MsgDispose, "a") > 0)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
	if life.count("close") != 1 {
		t.Fatalf("close ran %d times, want 1", life.count("close"))
	}
	if sawDispose.Load() {
		t.Fatal("reclaim_dispose was already written when Close ran")
	}
}

func TestTable_ResetDisposeLogFollowsClose(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	var sawDispose atomic.Bool
	life := &lifecycle{}
	life.onClose = func() {
		sawDispose.Store(countKeyMsg(h.events(), MsgDispose, "a") > 0)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open: %v", err)
	}

	tab.Reset()
	if life.count("close") != 1 {
		t.Fatalf("Reset returned with close run %d times, want 1", life.count("close"))
	}
	if got := countKeyMsg(h.events(), MsgDispose, "a"); got != 1 {
		t.Fatalf("%d dispose lines after Reset, want 1", got)
	}
	if sawDispose.Load() {
		t.Fatal("reclaim_dispose was already written when Close ran")
	}
	if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("sequence %v, want Reset to sleep before it closes", got)
	}
}

func TestTable_StdlibImports(t *testing.T) {
	fset := token.NewFileSet()
	for _, name := range []string{"table.go"} {
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote %s: %v", imp.Path.Value, err)
			}
			if strings.Contains(path, ".") {
				t.Fatalf("%s imports %q, which is not the standard library", name, path)
			}
		}
	}
}

func TestTable_HashChangeProof(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 20 * time.Millisecond})
	var mu sync.Mutex
	var ended []string

	ctxA, cancelA := context.WithCancel(context.Background())
	endA := &namedEnd{name: "a", mu: &mu, ended: &ended}
	if _, err := tab.Open(ctxA, "a", recLogger(h), func() (any, error) {
		return endA, nil
	}, testCloseHook(endA.Close)); err != nil {
		t.Fatalf("open a: %v", err)
	}
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	endB := &namedEnd{name: "b", mu: &mu, ended: &ended}
	if _, err := tab.Open(ctxB, "b", recLogger(h), func() (any, error) {
		return endB, nil
	}, testCloseHook(endB.Close)); err != nil {
		t.Fatalf("open b: %v", err)
	}

	cancelA()
	waitKeyMsg(t, h, MsgDispose, "a")
	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ended) == 1 && ended[0] == "a"
	})
	if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("a sequence %v", got)
	}
	if got := keySeq(h.events(), "b"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind}) {
		t.Fatalf("b sequence %v, want the still-held key to be untouched by a's ending", got)
	}
}

func TestTable_OpenSharesIncarnation(t *testing.T) {
	tab := New(Config{})
	h := &recHandler{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first, err := tab.Open(ctx, "shared", recLogger(h), func() (any, error) { return ending(7, nil), nil }, Hooks{})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, err := tab.Open(ctx, "shared", recLogger(h), func() (any, error) {
		t.Error("create ran again")
		return nil, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if first != second || first.(*box).n != 7 {
		t.Fatalf("values differ: %#v %#v", first, second)
	}
}

func TestTable_ConfigGraceFixedAfterNew(t *testing.T) {
	cfg := Config{Grace: 37 * time.Millisecond}
	tab := New(cfg)
	cfg.Grace = time.Hour
	if got := tab.grace; got != 37*time.Millisecond {
		t.Fatalf("grace %v after Config write, want 37ms", got)
	}
}

func TestTable_TwoTablesDoNotShare(t *testing.T) {
	a := New(Config{})
	b := New(Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &recHandler{}
	first, err := a.Open(ctx, "shared", recLogger(h), func() (any, error) { return ending(1, nil), nil }, Hooks{})
	if err != nil {
		t.Fatalf("open a: %v", err)
	}
	second, err := b.Open(ctx, "shared", recLogger(h), func() (any, error) { return ending(2, nil), nil }, Hooks{})
	if err != nil {
		t.Fatalf("open b: %v", err)
	}
	if first == second {
		t.Fatal("two tables stored the same value for the same key")
	}
}

func TestTable_OpenNilContextPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	tab := New(Config{Grace: time.Millisecond})
	//nolint:staticcheck // the nil context is the case under test
	_, _ = tab.Open(nil, "a", recLogger(&recHandler{}), func() (any, error) { return ending(1, nil), nil }, Hooks{})
}

func TestTable_OpenBackgroundDoesNotPanic(t *testing.T) {
	tab := New(Config{Grace: time.Millisecond})
	if _, err := tab.Open(context.Background(), "a", recLogger(&recHandler{}), func() (any, error) {
		return ending(1, nil), nil
	}, Hooks{}); err != nil {
		t.Fatalf("open: %v", err)
	}
}

func TestTable_HolderWithoutDoneChannelIsPolled(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 5 * time.Millisecond})
	holder := &nilDoneCtx{}
	life := &lifecycle{}
	if _, err := tab.Open(holder, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if life.count("sleep") != 0 {
		t.Fatal("a holder whose Done is nil was treated as gone")
	}
	holder.cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
	if got := life.seq(); !reflect.DeepEqual(got, []string{"sleep", "close"}) {
		t.Fatalf("lifecycle %v", got)
	}
}

func TestTable_ValueWithoutLifecycleMethodsStillDisposes(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	value, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return "a bare string", nil }, Hooks{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if value.(string) != "a bare string" {
		t.Fatalf("value %#v", value)
	}
	cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
	if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("sequence %v", got)
	}
}

func TestTable_ValueWithOnlyCloseIsUnaffected(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 5 * time.Millisecond})
	value := &onlyCloser{}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return value, nil }, testCloseHook(value.Close)); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
	if got := value.closes.Load(); got != 1 {
		t.Fatalf("Close ran %d times, want 1", got)
	}
}

func TestTable_ResetLogsOrphanThenDisposeAndKeepsNextIncarnation(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	staleCtx, cancelStale := context.WithCancel(context.Background())
	life := &lifecycle{}
	if _, err := tab.Open(staleCtx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open 1: %v", err)
	}

	tab.Reset()
	if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("Reset sequence %v, want sleep before close", got)
	}
	if got := life.seq(); !reflect.DeepEqual(got, []string{"sleep", "close"}) {
		t.Fatalf("Reset lifecycle %v", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	next := &lifecycle{}
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return next, nil }, testLifeHooks(next)); err != nil {
		t.Fatalf("open 2: %v", err)
	}
	// The stale holder's watcher now fires; it must not touch the new incarnation.
	cancelStale()
	time.Sleep(50 * time.Millisecond)
	if next.count("sleep") != 0 {
		t.Fatal("a stale holder drop slept the new incarnation")
	}
	if got := countKeyMsg(h.events(), MsgDispose, "a"); got != 1 {
		t.Fatalf("%d dispose lines, want 1", got)
	}
}

func TestTable_ResetStopsGraceWait(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	ctx, cancel := context.WithCancel(context.Background())
	life := &lifecycle{}
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgOrphan, "a")

	tab.Reset()
	waitKeyMsg(t, h, MsgDispose, "a")
	if got := countKeyMsg(h.events(), MsgDispose, "a"); got != 1 {
		t.Fatalf("%d dispose lines, want 1", got)
	}
	if got := life.count("sleep"); got != 1 {
		t.Fatalf("sleep ran %d times, want 1 (Reset must not sleep a sleeping value again)", got)
	}
	if mappedKeys(tab) != 0 {
		t.Fatal("Reset left a key stored")
	}
}

func TestTable_ConcurrentOpenSameKeySharesOneIncarnation(t *testing.T) {
	const openers = 8
	h := &recHandler{}
	tab := New(Config{Grace: time.Millisecond})
	var created atomic.Int32
	cancels := make([]context.CancelFunc, openers)
	values := make([]any, openers)
	var wg sync.WaitGroup
	for i := 0; i < openers; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, ctx context.Context) {
			defer wg.Done()
			value, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
				created.Add(1)
				return ending(1, nil), nil
			}, Hooks{})
			if err != nil {
				t.Errorf("open %d: %v", i, err)
			}
			values[i] = value
		}(i, ctx)
	}
	wg.Wait()
	if n := created.Load(); n != 1 {
		t.Fatalf("create ran %d times, want 1", n)
	}
	for i := range values {
		if values[i] != values[0] {
			t.Fatalf("open %d returned a different value", i)
		}
	}
	for _, cancel := range cancels {
		cancel()
	}
	waitKeyMsg(t, h, MsgDispose, "a")
	if got := countKeyMsg(h.events(), MsgDispose, "a"); got != 1 {
		t.Fatalf("%d dispose lines, want 1", got)
	}
}

func TestTable_NilTableOpenErrors(t *testing.T) {
	var tab *Table
	if _, err := tab.Open(context.Background(), "a", recLogger(&recHandler{}), func() (any, error) {
		return ending(1, nil), nil
	}, Hooks{}); err == nil {
		t.Fatal("no error from a nil table")
	}
}

func TestTable_ZeroValueOpenErrors(t *testing.T) {
	tab := &Table{}
	if _, err := tab.Open(context.Background(), "a", recLogger(&recHandler{}), func() (any, error) {
		return ending(1, nil), nil
	}, Hooks{}); err == nil {
		t.Fatal("no error from a zero-value table")
	}
}

func TestTable_NilOpenLoggerRejected(t *testing.T) {
	tab := New(Config{Grace: time.Millisecond})
	if _, err := tab.Open(context.Background(), "a", nil, func() (any, error) { return ending(1, nil), nil }, Hooks{}); err == nil {
		t.Fatal("no error from a nil logger")
	}
	if mappedKeys(tab) != 0 {
		t.Fatal("a rejected Open stored an incarnation")
	}
}

func TestTable_ConcurrentCancelLastHolders(t *testing.T) {
	const holders = 8
	h := &recHandler{}
	tab := New(Config{Grace: 5 * time.Millisecond})
	life := &lifecycle{}
	cancels := make([]context.CancelFunc, holders)
	for i := 0; i < holders; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return life, nil }, testLifeHooks(life)); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	for _, cancel := range cancels {
		wg.Add(1)
		go func(cancel context.CancelFunc) { defer wg.Done(); cancel() }(cancel)
	}
	wg.Wait()

	waitKeyMsg(t, h, MsgDispose, "a")
	if got := countKeyMsg(h.events(), MsgOrphan, "a"); got != 1 {
		t.Fatalf("%d orphan lines, want 1", got)
	}
	if got := countKeyMsg(h.events(), MsgDispose, "a"); got != 1 {
		t.Fatalf("%d dispose lines, want 1", got)
	}
	if got := life.count("sleep"); got != 1 {
		t.Fatalf("sleep ran %d times, want 1", got)
	}
}

func TestTable_HolderCountReleased(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	keepCtx, keepCancel := context.WithCancel(context.Background())
	defer keepCancel()
	if _, err := tab.Open(keepCtx, "a", recLogger(h), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
		t.Fatalf("open keep-alive: %v", err)
	}
	s := mustSlot(t, tab, "a")

	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { t.Error("create ran again"); return nil, nil }, Hooks{}); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		cancel()
	}
	waitUntil(t, func() bool { return holderCount(tab, s) == 1 })
}

func TestTable_ManyKeysDisposeIndependently(t *testing.T) {
	const keys = 20
	h := &recHandler{}
	tab := New(Config{Grace: 5 * time.Millisecond})
	cancels := make([]context.CancelFunc, keys)
	for i := 0; i < keys; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		if _, err := tab.Open(ctx, "k"+strconv.Itoa(i), recLogger(h), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	cancels[3]()
	waitKeyMsg(t, h, MsgDispose, "k3")
	if got := countMsg(h.events(), MsgDispose); got != 1 {
		t.Fatalf("%d dispose lines across every key, want 1", got)
	}
	waitUntil(t, func() bool { return mappedKeys(tab) == keys-1 })
}

func TestTable_OrphanAndDisposeUseTheLastBindingLogger(t *testing.T) {
	first := &recHandler{}
	second := &recHandler{}
	tab := New(Config{Grace: 5 * time.Millisecond})

	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", recLogger(first), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx2, "a", recLogger(second), func() (any, error) { t.Error("create ran again"); return nil, nil }, Hooks{}); err != nil {
		t.Fatalf("open 2: %v", err)
	}

	cancel1()
	cancel2()
	waitKeyMsg(t, second, MsgDispose, "a")
	if got := countKeyMsg(first.events(), MsgOrphan, "a"); got != 0 {
		t.Fatal("orphan landed on the first Open's logger")
	}
	if got := countKeyMsg(first.events(), MsgDispose, "a"); got != 0 {
		t.Fatal("dispose landed on the first Open's logger")
	}
	if got := countKeyMsg(first.events(), MsgPut, "a"); got != 1 {
		t.Fatal("the first Open lost its own put line")
	}
}

func TestTable_ReclaimRacesExpiry(t *testing.T) {
	const rounds = 40
	for round := 0; round < rounds; round++ {
		h := &recHandler{}
		tab := New(Config{Grace: 3 * time.Millisecond})
		var created atomic.Int32
		ctx1, cancel1 := context.WithCancel(context.Background())
		first, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) {
			created.Add(1)
			return &counterClose{}, nil
		}, Hooks{})
		if err != nil {
			t.Fatalf("round %d open 1: %v", round, err)
		}
		cancel1()

		ctx2, cancel2 := context.WithCancel(context.Background())
		second, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) {
			created.Add(1)
			return &counterClose{}, nil
		}, Hooks{})
		if err != nil {
			t.Fatalf("round %d open 2: %v", round, err)
		}

		// Either side of the grace edge is correct, but the caller always gets the stored value
		// and a value that is still held is never closed.
		if second == first {
			if created.Load() != 1 {
				t.Fatalf("round %d: reclaimed the incarnation but create ran %d times", round, created.Load())
			}
		} else if created.Load() != 2 {
			t.Fatalf("round %d: new incarnation but create ran %d times", round, created.Load())
		}
		if got := second.(*counterClose).closes.Load(); got != 0 {
			t.Fatalf("round %d: the returned value had been closed %d times", round, got)
		}
		cancel2()
		tab.Reset()
	}
}

func TestTable_ZeroGraceOpenRacesCancel(t *testing.T) {
	const rounds = 40
	for round := 0; round < rounds; round++ {
		tab := New(Config{Grace: 0})
		h := &recHandler{}
		ctx1, cancel1 := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx1, "a", recLogger(h), func() (any, error) { return &counterClose{}, nil }, Hooks{}); err != nil {
			t.Fatalf("round %d open 1: %v", round, err)
		}

		ctx2, cancel2 := context.WithCancel(context.Background())
		var second any
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); cancel1() }()
		go func() {
			defer wg.Done()
			value, err := tab.Open(ctx2, "a", recLogger(h), func() (any, error) { return &counterClose{}, nil }, Hooks{})
			if err != nil {
				t.Errorf("round %d open 2: %v", round, err)
			}
			second = value
		}()
		wg.Wait()

		if second == nil {
			t.Fatalf("round %d: Open returned no value", round)
		}
		if got := second.(*counterClose).closes.Load(); got != 0 {
			t.Fatalf("round %d: the returned value had been closed %d times", round, got)
		}
		cancel2()
		tab.Reset()
	}
}

func TestTable_ResetRacingOpenClosesEveryValue(t *testing.T) {
	const rounds = 50
	for round := 0; round < rounds; round++ {
		tab := New(Config{Grace: time.Millisecond})
		h := &recHandler{}
		var mu sync.Mutex
		var made []*counterClose

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			var stored *counterClose
			_, _ = tab.Open(ctx, "a", recLogger(h), func() (any, error) {
				stored = &counterClose{}
				mu.Lock()
				made = append(made, stored)
				mu.Unlock()
				return stored, nil
			}, Hooks{Close: func() {
				if stored != nil {
					stored.Close()
				}
			}})
		}()
		go func() { defer wg.Done(); tab.Reset() }()
		wg.Wait()

		cancel()
		tab.Reset()

		mu.Lock()
		values := append([]*counterClose(nil), made...)
		mu.Unlock()
		for i, value := range values {
			waitUntil(t, func() bool { return value.closes.Load() >= 1 })
			if got := value.closes.Load(); got != 1 {
				t.Fatalf("round %d value %d closed %d times, want exactly 1", round, i, got)
			}
		}
		if mappedKeys(tab) != 0 {
			t.Fatalf("round %d: a reset table still stores %d keys", round, mappedKeys(tab))
		}
	}
}

// TestTable_CancellableHoldersDoNotParkWaiters proves live WithCancel holders do not park a
// waiter for the hold. Same slack as TestTable_GoroutinesReturnToBaseline.
func TestTable_CancellableHoldersDoNotParkWaiters(t *testing.T) {
	const keys = 50
	const slack = 2
	base := settledGoroutines()

	tab := New(Config{Grace: 2 * time.Millisecond})
	h := &recHandler{}
	cancels := make([]context.CancelFunc, keys)
	for i := 0; i < keys; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		if _, err := tab.Open(ctx, "k"+strconv.Itoa(i), recLogger(h), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	if got := settledGoroutines(); got > base+slack {
		t.Fatalf("hold goroutines %d, baseline %d, slack %d", got, base, slack)
	}
	for _, cancel := range cancels {
		cancel()
	}
	waitUntil(t, func() bool { return countMsg(h.events(), MsgDispose) == keys })
}

func TestTable_GoroutinesReturnToBaseline(t *testing.T) {
	const keys = 50
	const slack = 2
	base := settledGoroutines()

	tab := New(Config{Grace: 2 * time.Millisecond})
	h := &recHandler{}
	cancels := make([]context.CancelFunc, keys)
	for i := 0; i < keys; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		if _, err := tab.Open(ctx, "k"+strconv.Itoa(i), recLogger(h), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
	for _, cancel := range cancels {
		cancel()
	}
	waitUntil(t, func() bool { return countMsg(h.events(), MsgDispose) == keys })
	waitUntil(t, func() bool { return runtime.NumGoroutine() <= base+slack })
}

func TestTable_ResetNil(_ *testing.T) {
	var tab *Table
	tab.Reset()
}

func TestTable_OpenLoggerLevelGatesPutDispose(t *testing.T) {
	quiet := &levelGate{min: slog.LevelInfo}
	tab := New(Config{Grace: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "quiet", recLogger(quiet), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
		t.Fatalf("open quiet: %v", err)
	}
	cancel()
	time.Sleep(60 * time.Millisecond)
	if got := countKeyMsg(quiet.events(), MsgPut, "quiet"); got != 0 {
		t.Fatalf("%d put lines through an info logger, want 0", got)
	}
	if got := countKeyMsg(quiet.events(), MsgDispose, "quiet"); got != 0 {
		t.Fatalf("%d dispose lines through an info logger, want 0", got)
	}

	loud := &levelGate{min: slog.LevelDebug}
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx2, "loud", recLogger(loud), func() (any, error) { return &lifecycle{}, nil }, Hooks{}); err != nil {
		t.Fatalf("open loud: %v", err)
	}
	cancel2()
	waitKeyMsg(t, &loud.recHandler, MsgDispose, "loud")
	if got := countKeyMsg(loud.events(), MsgPut, "loud"); got != 1 {
		t.Fatalf("%d put lines through a debug logger, want 1", got)
	}
}

func TestTable_CreatePanicUnsticksKey(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	func() {
		defer func() { _ = recover() }()
		_, err := tab.Open(context.Background(), "a", recLogger(h), func() (any, error) {
			panic("create boom")
		}, Hooks{})
		want := fmt.Sprintf("reclaim: create %q: panic: %v", "a", "create boom")
		if err == nil || err.Error() != want {
			t.Fatalf("create panic: err %v, want %s", err, want)
		}
	}()
	created := 0
	value, err := openReturned(context.Background(), t, tab, "a", recLogger(h), func() (any, error) {
		created++
		return unstuckValue, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	if value != unstuckValue || created != 1 {
		t.Fatalf("second open value %v created %d, want next created once", value, created)
	}
}

func TestTable_NilCreateUnsticksKey(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	func() {
		defer func() { _ = recover() }()
		_, err := tab.Open(context.Background(), "a", recLogger(h), nil, Hooks{})
		want := fmt.Sprintf("reclaim: create %q: nil create", "a")
		if err == nil || err.Error() != want {
			t.Fatalf("nil create: err %v, want %s", err, want)
		}
	}()
	value, err := openReturned(context.Background(), t, tab, "a", recLogger(h), func() (any, error) {
		return unstuckValue, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	if value != unstuckValue {
		t.Fatalf("second open value %v, want next", value)
	}
}

func TestTable_SleepPanicAfterFuncDoesNotCrash(t *testing.T) {
	if os.Getenv("RECLAIM_SLEEP_PANIC_CHILD") == "1" {
		var closed atomic.Int32
		h := &recHandler{}
		tab := New(Config{Grace: graceNoRace})
		ctx, cancel := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
			return "v", nil
		}, Hooks{
			Sleep: func() { panic("sleep boom") },
			Close: func() { closed.Add(1) },
		}); err != nil {
			os.Exit(2)
		}
		cancel()
		deadline := time.Now().Add(waitBudget)
		for time.Now().Before(deadline) && closed.Load() == 0 {
			time.Sleep(time.Millisecond)
		}
		if closed.Load() != 1 {
			os.Exit(3)
		}
		if _, err := tab.Open(context.Background(), "a", recLogger(h), func() (any, error) {
			return unstuckValue, nil
		}, Hooks{}); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestTable_SleepPanicAfterFuncDoesNotCrash$") //nolint:gosec // G204: re-exec this test binary
	cmd.Env = append(os.Environ(), "RECLAIM_SLEEP_PANIC_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("AfterFunc Sleep panic crashed the process: %v\n%s", err, out)
	}
}

func TestTable_ClosePanicAfterFuncDoesNotCrash(t *testing.T) {
	if os.Getenv("RECLAIM_CLOSE_PANIC_CHILD") == "1" {
		h := &recHandler{}
		tab := New(Config{Grace: 0})
		ctx, cancel := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
			return "v", nil
		}, Hooks{Close: func() { panic("close boom") }}); err != nil {
			os.Exit(2)
		}
		cancel()
		deadline := time.Now().Add(waitBudget)
		for time.Now().Before(deadline) && mappedKeys(tab) != 0 {
			time.Sleep(time.Millisecond)
		}
		if _, err := tab.Open(context.Background(), "a", recLogger(h), func() (any, error) {
			return unstuckValue, nil
		}, Hooks{}); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestTable_ClosePanicAfterFuncDoesNotCrash$") //nolint:gosec // G204: re-exec this test binary
	cmd.Env = append(os.Environ(), "RECLAIM_CLOSE_PANIC_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("AfterFunc Close panic crashed the process: %v\n%s", err, out)
	}
}

func TestTable_ClosePanicAfterFuncDoesNotCrashEnforce(t *testing.T) {
	if os.Getenv("RECLAIM_CLOSE_PANIC_ENFORCE_CHILD") == "1" {
		h := &recHandler{}
		tab := New(Config{Grace: 0})
		ctx, cancel := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
			return "v", nil
		}, Hooks{Close: func() { panic("close boom") }, EnforceCloseBeforeOpen: true}); err != nil {
			os.Exit(2)
		}
		cancel()
		deadline := time.Now().Add(waitBudget)
		for time.Now().Before(deadline) && mappedKeys(tab) != 0 {
			time.Sleep(time.Millisecond)
		}
		if _, err := tab.Open(context.Background(), "a", recLogger(h), func() (any, error) {
			return unstuckValue, nil
		}, Hooks{}); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestTable_ClosePanicAfterFuncDoesNotCrashEnforce$") //nolint:gosec // G204: re-exec this test binary
	cmd.Env = append(os.Environ(), "RECLAIM_CLOSE_PANIC_ENFORCE_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("AfterFunc Close panic crashed the process: %v\n%s", err, out)
	}
}

func TestTable_WakePanicReturnsErrorAndUnsticks(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	var closed atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
		return "v", nil
	}, Hooks{
		Wake:  func() { panic("wake boom") },
		Close: func() { closed.Add(1) },
	}); err != nil {
		t.Fatalf("first open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgOrphan, "a")
	func() {
		defer func() { _ = recover() }()
		_, err := tab.Open(context.Background(), "a", recLogger(h), func() (any, error) {
			return "ignored", nil
		}, Hooks{})
		want := fmt.Sprintf("reclaim: wake %q: panic: %v", "a", "wake boom")
		if err == nil || err.Error() != want {
			t.Fatalf("wake panic: err %v, want %s", err, want)
		}
	}()
	waitUntil(t, func() bool { return closed.Load() == 1 })
	value, err := openReturned(context.Background(), t, tab, "a", recLogger(h), func() (any, error) {
		return unstuckValue, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	if value != unstuckValue {
		t.Fatalf("third open value %v, want next", value)
	}
}

func TestTable_SleepPanicIsReportedAtError(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return "v", nil }, Hooks{
		Sleep: func() { panic("sleep boom") },
		Close: func() {},
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgHookPanic, "a")

	lines := h.hookPanics()
	if len(lines) != 1 {
		t.Fatalf("%d hook panic lines, want 1", len(lines))
	}
	got := lines[0]
	if got.level != slog.LevelError {
		t.Errorf("hook panic logged at %v, want error", got.level)
	}
	if got.hook != "sleep" || got.panic != "sleep boom" || got.key != "a" {
		t.Errorf("hook panic line %+v, want key a hook sleep panic sleep boom", got)
	}
	// Sleep never returned, so the incarnation was not parked asleep.
	if n := countKeyMsg(h.events(), MsgOrphan, "a"); n != 0 {
		t.Errorf("%d orphan lines after a Sleep panic, want 0", n)
	}
}

func TestTable_ClosePanicIsReportedAtErrorAndStillDisposes(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return "v", nil }, Hooks{
		Close: func() { panic("close boom") },
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgHookPanic, "a")
	waitKeyMsg(t, h, MsgDispose, "a")

	lines := h.hookPanics()
	if len(lines) != 1 {
		t.Fatalf("%d hook panic lines, want 1", len(lines))
	}
	got := lines[0]
	if got.level != slog.LevelError {
		t.Errorf("hook panic logged at %v, want error", got.level)
	}
	if got.hook != "close" || got.panic != "close boom" {
		t.Errorf("hook panic line %+v, want hook close panic close boom", got)
	}
}

func TestTable_ClosePanicIsReportedAtErrorAndStillDisposesEnforce(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return "v", nil }, Hooks{
		Close:                  func() { panic("close boom") },
		EnforceCloseBeforeOpen: true,
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgHookPanic, "a")
	waitKeyMsg(t, h, MsgDispose, "a")

	lines := h.hookPanics()
	if len(lines) != 1 {
		t.Fatalf("%d hook panic lines, want 1", len(lines))
	}
	got := lines[0]
	if got.level != slog.LevelError {
		t.Errorf("hook panic logged at %v, want error", got.level)
	}
	if got.hook != "close" || got.panic != "close boom" {
		t.Errorf("hook panic line %+v, want hook close panic close boom", got)
	}
}

func TestTable_WakePanicIsReportedAsErrorNotLogged(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) { return "v", nil }, Hooks{
		Wake:  func() { panic("wake boom") },
		Close: func() {},
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgOrphan, "a")

	_, err := tab.Open(context.Background(), "a", recLogger(h), func() (any, error) {
		return unstuckValue, nil
	}, Hooks{})
	if err == nil {
		t.Fatal("wake panic did not surface as an Open error")
	}
	// Wake had a caller to answer, so the panic travels as that error and is not logged twice.
	if n := len(h.hookPanics()); n != 0 {
		t.Errorf("%d hook panic lines for a Wake panic, want 0 (it is the Open error)", n)
	}
}

func TestTable_WakePanicWaiterReceivesErrorWithEnforce(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-releaseClose:
		default:
			close(releaseClose)
		}
	})
	wakeStarted := make(chan struct{})
	releaseWake := make(chan struct{})
	var created atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) { return firstIncarnation, nil }, Hooks{
		Wake: func() {
			close(wakeStarted)
			<-releaseWake
			panic("wake boom")
		},
		Close:                  func() { close(closeEntered); <-releaseClose },
		EnforceCloseBeforeOpen: true,
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgOrphan, "k")

	reclaimErr := make(chan error, 1)
	go func() {
		_, err := tab.Open(context.Background(), "k", recLogger(h), func() (any, error) {
			return "unreachable", nil
		}, Hooks{})
		reclaimErr <- err
	}()
	select {
	case <-wakeStarted:
	case <-time.After(waitBudget):
		t.Fatal("Wake did not start")
	}

	waiterErr := make(chan error, 1)
	go func() {
		_, err := tab.Open(context.Background(), "k", recLogger(h), func() (any, error) {
			created.Store(true)
			return nextIncarnation, nil
		}, Hooks{})
		waiterErr <- err
	}()
	// Wake still holds slotBusy; this Open must park on that ready before Wake panics.
	time.Sleep(50 * time.Millisecond)

	close(releaseWake)
	select {
	case <-closeEntered:
	case <-time.After(waitBudget):
		t.Fatal("Close did not run after the Wake panic")
	}

	want := fmt.Sprintf("reclaim: wake %q: panic: %v", "k", "wake boom")
	select {
	case err := <-waiterErr:
		if created.Load() {
			t.Fatal("waiter created while Close of the panicking Wake was blocked")
		}
		if err == nil || err.Error() != want {
			t.Fatalf("waiter: %v, want %s", err, want)
		}
	case <-time.After(waitBudget):
		t.Fatal("waiter did not receive the wrapping Wake error")
	}

	close(releaseClose)
	select {
	case err := <-reclaimErr:
		if err == nil || err.Error() != want {
			t.Fatalf("reclaiming Open: %v, want %s", err, want)
		}
	case <-time.After(waitBudget):
		t.Fatal("reclaiming Open did not return after Close returned")
	}
}
