// SetAlias, Watch, and ClearPublisher expose a public alias for a mapped key.
// Open holders stay on the ownership key; Watch subscribers are weak and receive
// Published when the alias binding changes.
package reclaim

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
)

// Watcher is a weak reference to a mapped value. Watch copies current into Value
// and does not increment holders, stop grace, or Wake.
type Watcher struct {
	Value *atomic.Value
}

// Box is the only type stored in a watcher atomic.Value. Yaegi panics if that
// Value's first Store type later changes (typed-nil *T vs *T, or LAPI vs AppSec).
type Box struct {
	Value any
}

// Published is what a watcher receives when an alias changes.
// Value is the mapped client, or the typed empty from Watch when nothing is published.
// The hook is func(any) holding this struct: Yaegi v0.16.1 panics on a typed hook
// argument and on asserting a foreign concrete value to an interface.
type Published struct {
	Value any
}

// aliasWatcher is one subscriber. changed is func(any) and receives Published.
type aliasWatcher struct {
	changed func(any)
	last    any
}

// aliasEntry is one public name: the incarnation it currently points at (or none)
// and the watchers that late-bind that name. alias and group are opaque to the table.
type aliasEntry struct {
	incarnation *slot
	publisher   string
	group       string
	empty       any
	watchers    []*aliasWatcher
}

// SetAlias publishes the mapped key under a public name. Holders stay on key;
// watchers attach to alias. A second publisher on the same alias is rejected.
// The same publisher may replace its own alias in the same group (rename).
func (t *Table) SetAlias(key, alias, publisher, group string) error {
	if t == nil || alias == "" || key == "" {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	incarnation := t.items[key]
	if incarnation == nil || (incarnation.state != slotAwake && incarnation.state != slotAsleep) {
		return fmt.Errorf("reclaim alias %q has no mapped key %q", alias, key)
	}
	if existing := t.aliases[alias]; existing != nil && existing.publisher != "" && existing.publisher != publisher {
		return fmt.Errorf("reclaim alias %q is held by %q; release it before %q can publish", alias, existing.publisher, publisher)
	}
	current := t.aliases[alias]
	for _, other := range t.aliases {
		if other == current || other.publisher != publisher || other.group != group {
			continue
		}
		t.clearAliasLocked(other)
	}
	entry := t.aliases[alias]
	if entry == nil {
		entry = &aliasEntry{}
		t.aliases[alias] = entry
	}
	if entry.incarnation != incarnation {
		t.detachAliasLocked(entry)
		incarnation.aliases = append(incarnation.aliases, entry)
		entry.incarnation = incarnation
	}
	entry.publisher = publisher
	entry.group = group
	t.storeWatchersLocked(entry, incarnation.value)
	return nil
}

// Watch registers changed for alias. It never waits for SetAlias and never binds a holder.
// First non-nil empty sticks. changed receives Published when the published value changes.
// A nil changed skips that call. The same value does not call it again.
// changed must not call back into this table: it runs while the table lock is held.
// When ctx is done, Watch drops this subscriber. That does not Close the mapped value.
func (t *Table) Watch(ctx context.Context, alias string, empty any, changed func(any)) {
	if ctx == nil {
		panic("reclaim: Watch requires a context")
	}
	if t == nil || alias == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entry := t.aliases[alias]
	if entry == nil {
		entry = &aliasEntry{empty: empty}
		t.aliases[alias] = entry
	}
	if entry.empty == nil && empty != nil {
		entry.empty = empty
	}
	watcher := &aliasWatcher{changed: changed}
	entry.watchers = append(entry.watchers, watcher)
	current := entry.empty
	if entry.incarnation != nil && !isNilValue(entry.incarnation.value) {
		current = entry.incarnation.value
	}
	t.deliverLocked(watcher, current)
	context.AfterFunc(ctx, func() { t.dropWatcher(alias, watcher) })
}

func (t *Table) dropWatcher(alias string, watcher *aliasWatcher) {
	if t == nil || alias == "" || watcher == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entry := t.aliases[alias]
	if entry == nil {
		return
	}
	kept := entry.watchers[:0]
	for _, other := range entry.watchers {
		if other != watcher {
			kept = append(kept, other)
		}
	}
	entry.watchers = kept
}

// ClearPublisher drops every alias this publisher still holds in group.
// Watchers stay registered for a later SetAlias.
func (t *Table) ClearPublisher(publisher, group string) {
	if t == nil || publisher == "" || group == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, entry := range t.aliases {
		if entry.publisher != publisher || entry.group != group {
			continue
		}
		t.clearAliasLocked(entry)
	}
}

func (t *Table) unbindIncarnationLocked(incarnation *slot) {
	if incarnation == nil {
		return
	}
	bound := incarnation.aliases
	incarnation.aliases = nil
	for _, entry := range bound {
		entry.incarnation = nil
		entry.publisher = ""
		entry.group = ""
		t.storeWatchersLocked(entry, entry.empty)
	}
}

func (t *Table) clearAliasLocked(entry *aliasEntry) {
	t.detachAliasLocked(entry)
	entry.publisher = ""
	entry.group = ""
	t.storeWatchersLocked(entry, entry.empty)
}

func (t *Table) detachAliasLocked(entry *aliasEntry) {
	incarnation := entry.incarnation
	if incarnation == nil {
		return
	}
	kept := incarnation.aliases[:0]
	for _, other := range incarnation.aliases {
		if other != entry {
			kept = append(kept, other)
		}
	}
	incarnation.aliases = kept
	entry.incarnation = nil
}

func (t *Table) storeWatchersLocked(entry *aliasEntry, current any) {
	value := current
	if isNilValue(current) {
		value = entry.empty
	}
	for _, watcher := range entry.watchers {
		t.deliverLocked(watcher, value)
	}
}

// deliverLocked calls changed with Published when value differs from the last delivery.
// Nil and a typed nil are the same value, so the first empty watch does not call changed.
func (t *Table) deliverLocked(watcher *aliasWatcher, value any) {
	if watcher == nil || sameValue(watcher.last, value) {
		return
	}
	watcher.last = value
	if watcher.changed == nil {
		return
	}
	if recovered := runHookValue(watcher.changed, Published{Value: value}); recovered != nil {
		return
	}
}

func runHookValue(hook func(any), value any) (recovered any) {
	if hook == nil {
		return nil
	}
	defer func() { recovered = recover() }()
	hook(value)
	return nil
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Ptr && reflected.IsNil()
}

func sameValue(left, right any) bool {
	if isNilValue(left) && isNilValue(right) {
		return true
	}
	if isNilValue(left) || isNilValue(right) {
		return false
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if leftValue.Kind() == reflect.Ptr && rightValue.Kind() == reflect.Ptr {
		return leftValue.Pointer() == rightValue.Pointer()
	}
	return left == right
}
