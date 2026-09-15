## 1. Wrapper

- [x] 1.1 Route `binColumn` `country_short` through `usableMeta` (do not emit `XX` from BIN).

## 2. Tests

- [x] 2.1 Flip `a bin source answers - for an unknown address, so XX does not apply` to expect `XX` (and not marking the hop locked).
- [x] 2.2 `allowedCountries: [XX]` allows a BIN-only miss.
- [x] 2.3 Merged catalog: BIN `-` then a later source fills country.

## 3. Docs

- [x] 3.1 README: drop the BIN `-` counts-as-a-country note.
- [x] 3.2 Usage packets `knowledge/devdocs/core_geoblock_plugin_request-mode.md` and `knowledge/devdocs/core_geoblock_database_lookup.md`.
