## Context

See proposal.md — Why. `enrich` calls `writePublicLookupHeaders(req, rec, &written)` and only then checks the error `recordForLookup` returned, so anything in `rec.Country` is already on the header.

## Goals / Non-Goals

- `countryHeader` only ever carries the values the spec enumerates.
- `banIfError` behaviour is unchanged.
- **Non-goal:** reordering `enrich` so headers are written after the error check. That would leave the `PRIVATE` default in place for a failed hop, which is the fail-open #76 fixed.

## Decisions

- **Drop the country from the error records rather than reorder the caller.** Alternative: move the write below the `if err != nil` — rejected and measured. It leaves the `PRIVATE` default on the failed hop, so `mode: enrich` with `X-Forwarded-For: Norway` enriches as `PRIVATE`, a legal value meaning "internal address"; under `defaultAllow: false` the block stage then reads it as the `allowPrivate` verdict. That is #76's fail-open again. Letting the empty country fall into the existing `XX` branch avoids it. Letting the empty country fall into the existing `XX` branch reuses the mechanism #76 added and needs no new state.
- **`XX`, not a new sentinel.** An unparseable hop is a hop with no known country, which is what `XX` already means. It does not mark the country written, so a later resolving hop still wins — `DE, 8.8.8.8` ends `US`.
- **Empty country, not `XX` written directly in `recordForLookup`.** Returning `Record{Country: XX}` would take the normal write path and mark the country written, locking the header so a later resolving hop could not win — the alternative #76's own design rejected.
- **Both error paths, not just the unparseable one.** The `Lookup` error return has the same shape and the same caller. Its value must parse as an IP so it is not free-form, but an address is still not an ISO country.

## Risks / Trade-offs

- `countryHeader` changes value for unparseable hops, from the token to `XX`. Anything downstream keying on the token breaks — but it was keying on client-controlled input.
- With `defaultAllow: false` and `banIfError: false`, a chain whose first hop is a token naming an allowed country stops passing. That is the defect, not a regression.
- A chain whose hops all fail to parse still passes under `banIfError: false` (`Norway` alone is `pass:none`, before and after). Nothing denied, and that setting makes errors non-fatal. Out of scope here.

## Migration Plan

Ship. No config change. Operators who want the old permissiveness for failed lookups already have `banIfError: false` plus `allowedCountries: [XX]`.
