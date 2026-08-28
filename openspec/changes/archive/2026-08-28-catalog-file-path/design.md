## Context

See proposal.md — Why. Today `databaseDownloads` is `{ url, headers, databaseType, archive }`. Each provider still resolves a seed via `*_databaseFilePath` and `fileutils.Search` / `TRAEFIK_PLUGIN_GEOBLOCK_PATH`. `pkg/dbdownload.Latest` runs only when a pointer supplies a catalog key.

## Goals / Non-Goals

**Goals:**

- One Resolve in `pkg/dbdownload` for dated file, catalog `path`, and default-name search.
- Providers open the path Resolve returns. They do not search for seeds.
- Traefik Config seed keys and `databaseFilePath` are gone.

**Non-Goals:**

- Changing GET, unpack, dated-name, or ticker rules.
- Removing role pointers or inventing implicit catalog keys.
- Changing Lookup / Record.
- Replacing `TRAEFIK_PLUGIN_GEOBLOCK_PATH`.

## Decisions

- **`path` on the catalog row.** Matches explore. Alternative: keep prefixed Config keys — rejected; that leaves two owners.
- **Pointers stay.** A named `path` needs a key. Alternative: implicit default keys (`geo`, `lite`) — rejected; explore forbade inventing them.
- **Default filenames stay in vendor packages.** Plugin passes `DefaultFileName` on internal `dbdownload.Config`. Alternative: download owns names by `databaseType` — rejected; two BIN names.
- **No `databaseFilePath` alias.** Human: remove backwards-compatible alias. Alternative: copy onto geo catalog `path` — rejected.
- **ASN empty stays optional.** `AllowMissing` stays on the IP2Location ASN factory when the pointer and path are empty.

## Risks / Trade-offs

- [Operators who set only `*_databaseFilePath`] → Mitigation: README + usage: put `path` on a catalog row and set the pointer. Bundled default + env still works with no catalog.
- [Tests and compose still use seed keys] → Mitigation: rewrite those call sites in the same change.
- [Empty pointer never sees a catalog `path`] → Accepted. Documented. Named seed requires a pointer.

## Migration Plan

1. Move each `*_databaseFilePath` value onto `databaseDownloads.<key>.path` and set the matching pointer.
2. Delete `databaseFilePath`. There is no shim.
3. Keep `TRAEFIK_PLUGIN_GEOBLOCK_PATH` for bundled defaults.

## Open Questions

None. Explore rows are resolved.
