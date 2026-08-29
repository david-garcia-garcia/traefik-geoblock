## 1. Search and Resolve

- [x] 1.1 Replace `fileutils.Search` walk with exact joins `{env}/seeds/<name>` then `{env}/<name>`. Log unset env and missed exact paths.
- [x] 1.2 Resolve: WARN when catalog `path` is set and not a file; drop silent cwd `seeds/` fallbacks.
- [x] 1.3 ASN `DefaultFileName` empty in `asnBINConfig`.

## 2. Wrapper logs

- [x] 2.1 Scope BIN/MMDB logger with `key`. Remove updater `source` attr.
- [x] 2.2 Update fileutils, Resolve, BIN, plugin, and lifecycle tests that assumed a walk or env-root file without `seeds/`.

## 3. Docs

- [x] 3.1 README `TRAEFIK_PLUGIN_GEOBLOCK_PATH` and `knowledge/devdocs/core_geoblock_database_source.md`: exact paths, no walk, ASN not bundled.
