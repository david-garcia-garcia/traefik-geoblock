## 1. Config and shipped catalog

- [x] 1.1 Add `vendor`, `defaultFile`, `enabled *bool` on `DatabaseSource`. Remove `databaseProvider` and `*_source_*` pointers. Yaegi alias stays `geoblock.Config`.
- [x] 1.2 `Prepare` (lookup modes): insert reserved rows when absent; treat nil `enabled` as on; fail on unknown/empty `vendor`, format mismatch, or zero enabled rows.
- [x] 1.3 Resolve uses catalog `defaultFile` instead of vendor `DefaultFileName` constants.

## 2. Open and merge

- [x] 2.1 Open one wrapper per enabled row (`ip2location` / `ip2location-asn` BIN, `ipinfo` / `maxmind` MMDB).
- [x] 2.2 Merge Lookup in lexicographic key order; first non-empty meta key wins; skip source errors; all-error returns error.
- [x] 2.3 `bindDatabase` / `NewCore` open merged sources only for `enrich` and `enrichandblock`.

## 3. Tests and operator docs

- [x] 3.1 Replace provider/pointer tests with catalog `vendor` / `enabled` / merge / shipped-row cases.
- [x] 3.2 README, compose, `.traefik.yml`: catalog-only examples; no `databaseProvider` or pointers.
