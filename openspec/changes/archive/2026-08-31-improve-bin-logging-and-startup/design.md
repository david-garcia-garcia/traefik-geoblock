## Context

See proposal.md — Why. `SourcePath()` already holds the catalog/seed file. Create runs once per reclaim singleton; the logger is the first `OpenBIN` caller’s slog (`plugin=<middleware>` from `NewBootstrap`).

## Goals / Non-Goals

**Goals:**
- Init and hot-swap lines name the original catalog/seed file.
- BIN lines name the creating middleware as `owner_plugin`, not `plugin`.
- Temp copy basename includes format, catalog key, and a Unix-nanosecond clock.

**Non-Goals:**
- Changing `Path()` / `SourcePath()` or Config.
- Renaming `plugin` on `logging.New` or request logs.
- MMDB (no temp copy).
- `reclaim_put` key shape.

## Decisions

- **Keep `path`, add `source_path`.** `path` stays the opened handle (`Path()`). The missing original is a new attr. Same pair on hot-swap (`new_path` + `source_path`). Alternative: replace `path` with the source — rejected (explore; greppers who want the handle still have `path=`).
- **`NewOwner` at the BIN call site.** Catalog BIN open uses a text logger with `owner_plugin` and no `plugin`. `newBIN` still adds `key`. Alternative: wrap slog to rename handler-baked `plugin` — rejected (handler attrs cannot be stripped). Alternative: rename `logging.New` — rejected (every middleware line).
- **`bin_<safeKey>_<unixNano>.BIN`.** Safe token keeps `[A-Za-z0-9._-]`; other runes become `_`. Empty key → `bin_<unixNano>.BIN`. Alternative: `IP2LOCATION` + date + time + nsec — rejected (long, no key). Alternative: `CreateTemp` random — rejected (operator asked for a real timestamp).

## Risks / Trade-offs

- [Dashboards match `plugin=` on `BIN initialized`] → Mitigation: attr rename is intentional; `owner_plugin` is the same middleware string.
- [Same-nanosecond name collision] → Mitigation: `Copy` does not overwrite; a second create in the same nanosecond fails and initialize falls back to opening the source (existing copy-fail path).

## Migration Plan

Ship in the next plugin build. No config migration. After upgrade, grep `BIN initialized` for `source_path=` and `owner_plugin=`; temp files match `bin_<key>_*.BIN`.
