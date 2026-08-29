## Why

Traefik calls plugin `New` once per router (and again on every applied reload). This plugin has one config, so N routers build N `Plugin` values. The BIN/MMDB file is already shared. The leftover cost is constructing the Plugin itself (maps, regexes, IP helpers, logger, provider object) every time.

## What Changes

- `New` stores one `Plugin` (next unset) on the process reclaim table, keyed by middleware name plus a hash of the normalized `Config`.
- A later `New` with the same name and config shallow-copies that Plugin and `ForRoute`s this `next`. `ForRoute` opens the DatabaseProvider so wrappers bind this `New` context.
- `New` still returns `*Plugin`. No new handler type.
- Plugin dispose is empty. After grace the table drops the incarnation.
- Table `Open` dispose-vs-lifetime-context is **not** this change.

## Capabilities

### New Capabilities

- `core_geoblock_plugin_instance-reclaim`: one Plugin incarnation per name+config hash; bind `New` ctx; reclaim across reload; unreclaimed drop after grace.

### Modified Capabilities

None.

## Impact

- Root `plugin.go` `New` (reclaim). `pkg/geoblock` Prepare / NewCore / ServeHTTP
- Process `reclaim` table (`plugin:` key prefix)
- Tests: root `plugin_instance_test.go` (share, next, miss, reclaim / grace). Wrapper lifecycle stays in `pkg/geoblock`
- Existing wrapper lifecycle tests still bind wrappers on every `geoblock.New`
- Usage: packets for Plugin packages and instance reclaim
