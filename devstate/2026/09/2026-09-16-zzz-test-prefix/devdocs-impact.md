# Devdocs impact
change: zzz-test-file-prefix

## Units
- Package test — pattern — `knowledge/devdocs/core_geoblock_test-harness.md`
- Plugin incarnation tests — pattern — `knowledge/devdocs/core_geoblock_plugin_instance.md` Key files

## Findings
- [ ] stale-usage  Package test — `core_geoblock_test-harness.md` Key files and how-to-use still name `plugin_instance_test.go` / `plugin_*_test.go` without `zzz_`
  Why skipped: requirement Out of scope and explore assumed do not rewrite those packets in this change. `*_test.go` globs remain true.
- [ ] stale-usage  Plugin incarnation tests — `core_geoblock_plugin_instance.md` Key files still name `plugin_instance_test.go`
  Why skipped: same Out of scope / assumed line.
