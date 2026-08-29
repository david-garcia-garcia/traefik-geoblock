## Context

Resolve already owns dated file, catalog `path`, then bundled default. `Search` walks `TRAEFIK_PLUGIN_GEOBLOCK_PATH` for a basename. That walk hides `seeds/` in logs and ERRORS for ASN, which has no committed seed. Ban HTML uses the same helper (`geoblockban.html` at plugin root, not under `seeds/`).

## Goals / Non-Goals

**Goals:**

- Exact path joins from the plugin root env
- Catalog `key` on wrapper and source logs
- WARN when catalog `path` is set and missing
- No ASN bundled-name search

**Non-Goals:**

- Reclaim table logs on the plugin logger
- BIN working copies under `databaseAutoUpdateDir`
- Compatibility glob for old `*_IP2LOCATION-LITE-*.IPV6.BIN` names
- Walking an operator directory for a basename

## Decisions

- `Search(base, defaultFile)`: if `base` is an existing file, return it. Else require `TRAEFIK_PLUGIN_GEOBLOCK_PATH`. Try `{env}/seeds/{defaultFile}`, then `{env}/{defaultFile}`. First existing file wins. No `filepath.Walk`.
- Env unset: log that the env must be the plugin root. Env set and both misses: log both exact paths and that the env is probably not the plugin root.
- ASN `DefaultFileName` is empty. Resolve with no dated file and no path returns empty without Search.
- Wrapper logger: `logger.With("key", source.Key)` at open. Updater keeps that logger; do not add `source`.
- Drop silent cwd `./seeds/` candidates after Search. Tests set the env to a tree that contains `seeds/`.

## Risks / Trade-offs

- Tests that put a default file in a random subdirectory of the env dir will fail until they use `seeds/` or the env root. That is intended.
- An operator who dropped `IP2LOCATION-LITE-ASN.IPV6.BIN` somewhere under the env dir must set catalog `path` instead.

## Migration Plan

No Config key change. Operators who already set `TRAEFIK_PLUGIN_GEOBLOCK_PATH` to the plugin root keep working for geo / IPinfo / MaxMind / ban HTML. ASN waits for dated file or `path` without an ERROR walk.
