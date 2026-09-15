## Context

See proposal.md — Why. `initialize` copies dated `Latest` before `OpenDB`. `hotSwap` already copies a later dated file and replaces the handle. `Resolve` still returns dated first; do not change that order. Bundled seed is `defaultFile` via `fileutils.Search`.

## Goals / Non-Goals

**Goals:**
- `OpenBIN` returns without waiting on a dated temp copy when a bundled seed exists.
- Dated copy still happens; the live handle becomes that copy via `hotSwap`.
- Init and hot-swap lines include `size_bytes`.

**Non-Goals:**
- Changing `Resolve` order or adding operator config.
- Using catalog `path` as the startup handle.
- Temp-copying the seed.
- MMDB.
- Adding a mutex around `LookupRecord` / `hotSwap` unless this change creates a new race.

## Decisions

- **Seed-first only when `Search("", defaultFile)` succeeds.** Startup handle is the bundled file, not `path` (path may be the paid BIN). Alternative: treat `path` as seed — rejected (explore; path can be GB). Alternative: always async copy with no handle — rejected (`New` would have no lookup).
- **Leave `Resolve` alone.** Dated Latest stays the target. `initialize` asks Search for a seed beside that target. Alternative: a `ResolveSeed` that skips Latest — not needed if initialize calls Search.
- **Copy dated in a goroutine after the seed is open; then `hotSwap`.** Same `createLocalCopy` + `hotSwap` as the updater. Skip the swap if `Close` already ran. Alternative: sync copy after return via updater only — rejected (updater waits on MinAge / URL; a file already on disk would never swap).
- **`size_bytes` from `os.Stat` of the opened file** on both `BIN initialized` and `BIN hot-swapped`. Alternative: human string — rejected (explore; not greppable). Alternative: size of `source_path` — rejected (operator needs the handle they can `ls`).

## Risks / Trade-offs

- [Lookups during the seed window miss paid-only columns] → Accepted. Log that initialize opened the seed while the dated copy is pending.
- [No seed (ASN LITE, missing `defaultFile`)] → Sync dated copy, today’s path. `New` can still block on a GB file.
- [Swap after Close] → Check the wrapper is still open before `hotSwap`; ignore errors after dispose.
- [Existing `hotSwap` assigns `w.db` without a lock] → Do not add a lock unless tests show a new race from the startup goroutine.

## Migration Plan

Ship in the next plugin build. No config migration. After upgrade, `BIN initialized` for a row with dated + seed shows the seed path and `size_bytes`; `BIN hot-swapped` follows with the dated temp copy.
