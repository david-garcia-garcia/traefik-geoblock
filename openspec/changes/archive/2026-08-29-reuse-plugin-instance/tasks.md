## 1. New reuses the Plugin incarnation

- [x] 1.1 Add `plugin:` key (name + JSON+FNV of normalized Config) and `reclaim.Open` in `New`. Construct maps, regexes, IP helpers, logger, and ban HTML only in create. Always open the DatabaseProvider before Open. Shallow-copy the stored `*Plugin`, set `next` and this call’s provider. Empty dispose. Do not use `Table[*Plugin]`.
- [x] 1.2 Package tests: two `New` same name+config share incarnation pointers; different next handlers; name miss; allowed-country miss. Short grace.
- [x] 1.3 Package tests: cancel generation then `New` same name+config before grace (one `plugin:` put, reclaim); grace without `New` then later `New` (new put). Filter reclaim logs by `plugin:` prefix.

## 2. Usage

- [x] 2.1 Add `knowledge/devdocs/core_geoblock_plugin_instance.md` and index it. How `New` reuses a Plugin on the process table.
