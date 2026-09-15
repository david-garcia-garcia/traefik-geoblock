package reclaim

import (
	"context"
	"fmt"
	"log/slog"
)

// OpenTyped is OpenWithHooks with the stored value returned as T, so a caller that knows what it
// created does not repeat the same type assert on every Open. It is a function and not a method
// because Go does not allow a method to declare its own type parameter.
//
// Yaegi: the instantiation must stay a call expression in a package that can name T. Do not
// declare a package-level var, type alias, or struct field whose type names a generic
// instantiation from another package; Yaegi v0.16.1 cannot resolve those.
func OpenTyped[T any](ctx context.Context, t *Table, key string, logger *slog.Logger, create func() (any, Hooks, error)) (T, error) {
	var zero T
	stored, err := t.OpenWithHooks(ctx, key, logger, create)
	if err != nil {
		return zero, err
	}
	typed, ok := stored.(T)
	if !ok {
		return zero, fmt.Errorf("reclaim: open %q: want %T, got %T", key, zero, stored)
	}
	return typed, nil
}
