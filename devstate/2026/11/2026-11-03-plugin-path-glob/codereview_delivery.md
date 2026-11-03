# Delivery

## Motivation

The plugin reconstructs the on-disk plugin directory from `TRAEFIK_PLUGIN_GEOBLOCK_PATH` (`Search`) so `seeds/<defaultFile>` and `geoblockban.html` can be joined. Catalog open then decides whether a missing file is fatal or a wait-for-GET. Traefik unpacks catalog plugins under `{plugins-storage}/sources/gop-<random>/src/<module>` and does not hand that path to plugin code.

`Search` required an existing directory. A glob such as `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` failed as "is not a directory". Unset env logged ERROR `TRAEFIK_PLUGIN_GEOBLOCK_PATH must be set to the plugin root` with no try of that glob or `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Several matches had no rule for which directory contained `seeds/<defaultFile>`.

A catalog row with a URL still failed New when `defaultFile` was missing. BIN `AllowMissing` was true only when `Path` and `DefaultFileName` were both empty. MMDB hardcoded `allowMissing: false`. `initialize` returned `failed to resolve database path` / `database file not found` before keep-current could GET. The live url-download spec required that fail. `banIfError: false` while the handle is nil already skipped the lookup ban and continued into `decide()`, with no operator contract that this is fail-open on lookup only.

`chooseStartupTarget` still called `BundledFile` after a dated file had already won, and `Search` logged ERROR `bundled file not found; TRAEFIK_PLUGIN_GEOBLOCK_PATH is probably not the plugin root`. README Local plugin still named `/plugins-storage/sources/github.com/david-garcia-garcia/traefik-geoblock` with no `gop-*`.

Left alone, catalog deploys cannot find seeds without knowing the random `gop-*` name. URL-backed reserved rows never start keep-current. Operators treat a working dated open as a broken plugin root. Docs point at a path Traefik no longer writes.

Priority: P2 — real operator, admin-user, or end-user pain, with a workaround or limited blast radius

## Implementation

`Search` is the seam for plugin-directory location. An exact env directory wins and skips glob. A set env (glob or exact) is exclusive: `filepath.Glob` matches that are directories, zero matches do not fall through. Unset env tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Several matches keep the first lexicographic directory that contains `seeds/<that call's defaultFile>`, else `<defaultFile>` at that directory.

`openCatalogRow` sets allow-missing when the row has a URL or empty path and empty defaultFile. `MMDBConfig` gains `AllowMissing`. `initialize` WARNs `no database file yet; waiting for auto-update` and returns so keep-current still runs. Missing `defaultFile` with no URL still fails New. Nil-handle lookups stay `BIN is not open` / `MMDB is not open`.

`Search` WARNs a bundled-file miss that is the only remaining path (tried `seeds/` and root paths, env probably not the plugin directory) and WARNs an exclusive glob or directory miss. `chooseStartupTarget` peeks `BundledFile` only when `DefaultFileName` is set and DEBUGs a miss when Latest already won.

README names Traefik's `gop-*` unpack and documents `banIfError: false` as fail-open on lookup only. The usage packet and the url-download / source-catalog delta specs follow that Search and wait contract.

## What this changes
**Operators.** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` may be a glob or omitted (tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`; a set env does not fall through); a URL-backed catalog row starts with WARN `no database file yet; waiting for auto-update`; bundled miss after a dated file won is DEBUG; `banIfError` default stays true (false is fail-open on lookup only).

**Admin users.** None.

**Developers.** `Search` locates the plugin directory from an exact env path, an env glob, or those unset defaults; catalog URL wait (BIN and MMDB `AllowMissing`) starts with no on-disk file; a missing `defaultFile` with no URL still fails New.

**End users.** Until that download lands, lookups error and the default `banIfError: true` bans those requests.
