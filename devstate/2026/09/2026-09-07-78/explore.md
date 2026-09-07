# Explore
IssueKey: 2026-09-07-78

## Concepts

Three values, three owners. They must not collapse.

```
IP2Location country_short  ──►  Combined FillEmpty  ──►  writePublicLookupHeaders
         "-"                         ""                         "XX"
      this source                 field still empty          plugin sentinel
      has no country              next source may fill       not PRIVATE, not banIfError
```

- **Hop IP** is already owned by `IPHeaders` / `iplookup`. This change does not reconstruct identity.
- **Empty country on a Record** is owned by catalog merge. `usableMeta` maps IP2Location `-` (and empty / unavailable / invalid) to `""`. `FillEmpty` copies a field only when the current value is `""`.
- **`XX` (`UnknownCountryAlias`)** is owned by `writePublicLookupHeaders` after that hop’s *merged* lookup. Empty country writes `XX` without marking `written`, so a later hop that resolves still wins.

`binColumn` returns `country_short` **raw**. Every other BIN column already goes through `usableMeta`. Combined therefore treats a BIN miss as a resolved country `-`. The plugin writes `-` on `countryHeader`, marks the hop written, and `allowedCountries: [XX]` does not cover default (enabled BIN) catalogs.

Reproduced: `go test ./pkg/geoblock -run TestMode_UnresolvedPublicIPFollowsDefaultAllow/a_bin_source_answers` passes on `2026-09-07-78` at dest HEAD (`a20cd86` lineage). Subtest expects `X-Ipcountry` `-` for `203.0.113.7` on `seedCatalog(dbFilePath)`.

Do **not** have BIN emit `XX`. Combined would treat `XX` as a filled country, skip later sources, and lock the hop.

Dest already documents the split operators hit:

- README L421: a `bin` unknown “counts as a country”, so a catalog with one enabled does not produce `XX`.
- Usage `core_geoblock_database_lookup.md` / `core_geoblock_plugin_request-mode.md`: same split.
- Spec `core_geoblock_plugin_request-mode`: lookup SHALL write `XX` when enabled sources returned **no country**. BIN `-` is the hole in that SHALL.
- Spec `core_geoblock_database_lookup` “BIN Lookup applies mapped Get_all columns”: copy mapped paths; no requirement that `country_short` go through `usableMeta`.
- Archived #76 `design.md` listed routing `country_short` through `usableMeta` as a **non-goal** so it would not land with the PRIVATE fail-open.

Pester / compose do not assert header `-`. Only `plugin_mode_test.go` does.

```
DestBranch hop (enabled BIN, public miss)
  BIN country_short = "-"
  FillEmpty: "-" is non-empty → later MMDB/IPinfo cannot fill
  writePublicLookupHeaders: country != "" → write "-" , written=true
  later hop cannot replace
  allowedCountries [XX] misses

Intended hop
  BIN country_short through usableMeta → ""
  FillEmpty: later source may fill country
  if still empty: write XX, written stays false
  allowedCountries [XX] covers BIN-only and mmdb-only misses
```

## Decisions

- Catalog merge owns empty; plugin owns `XX`. One-line product change: `country_short` through `usableMeta` (same as sibling BIN columns). Plugin path already exists.
- Fold specs onto `core_geoblock_database_lookup` (BIN miss is empty so Combined can fill) and `core_geoblock_plugin_request-mode` (BIN-only miss is `XX`, not `-`; still does not mark written). No new spec leaf.
- Flip the #76 subtest that asserts `-`; add a merged-catalog case (BIN `-` then a later source’s country).
- Update README L421 and the two usage packets that currently teach “BIN `-` counts as a country” in the same change (operator contract of this ticket, not extra product).
- `banIfError`, `PRIVATE`, and MMDB sentinels stay as dest.

## Open questions

- Q: Who already owns hop identity (client address) versus country for that hop?
  Decision: resolved — hop IP is `IPHeaders` / `iplookup`. Country after merge is Combined / wrappers. Header sentinel `XX` is `writePublicLookupHeaders`. BIN MUST NOT emit `XX`. Reuse those outputs; do not reconstruct.
  By: explore

- Q: Do Pester or compose cases assert `X-Ipcountry` `-` for a BIN miss?
  Decision: resolved — no. Only `pkg/geoblock/plugin_mode_test.go` subtest `a bin source answers - for an unknown address, so XX does not apply`. README L421 documents the old operator note.
  By: explore

- Q: Should this change add a merged-catalog fallback test (BIN `-` then a later source fills country)?
  Decision: assumed — yes. Ticket Desired names it. One package test next to the flipped BIN-miss header case (`plugin_mode_test.go` or Combined lookup test).
  By: explore

- Q: Treat MMDB / IPinfo vendor miss sentinels (`-`, `ZZ`) as empty the same way?
  Decision: assumed — no. Ticket out of scope is BIN `country_short` through `usableMeta` only.
  By: explore

- Q: Fold the spec delta onto existing lookup + request-mode leaves, or open a new leaf?
  Decision: assumed — fold. `core_geoblock_plugin_request-mode` already requires `XX` when sources returned no country; `core_geoblock_database_lookup` already owns BIN column copy. FindSpecHost in propose confirms.
  By: explore
