# Devdocs impact
change: plugin-path-glob

## Units
- Search — pattern — `pkg/fileutils/fileutils.go` (`PluginPathEnv`, plugin directory glob)
- Source — subsystem — `pkg/dbsource/resolve.go`, spec `core_geoblock_database_url-download`
- Wrapper — subsystem — `pkg/dbwrappers/lifecycle.go`, `pkg/dbwrappers/mmdb.go`
- Database lookup — subsystem — `pkg/geoblock/plugin.go` `openCatalogRow`
- Request mode — subsystem — `pkg/geoblock` `BanIfError` / ServeHTTP
- Plugin packages — subsystem — `knowledge/devdocs/core_geoblock_plugin_packages.md` (Plugin root vs on-disk tree)

## Findings
- [x] language-gap  Search / Plugin directory — `core_geoblock_database_source` has How-to, no Language terms
- [x] stale-usage  Source — `core_geoblock_database_source` Key files omit `pkg/fileutils`
- [x] language-gap  AllowMissing — `core_geoblock_database_wrapper` has a How-to parenthetical, no Language term
- [x] stale-usage  Wrapper — `core_geoblock_database_wrapper` incomplete for MMDB `AllowMissing`, wait WARN, dated-won DEBUG
- [x] language-gap  BanIfError — `core_geoblock_plugin_request-mode` has a snippet, no Language term
- [x] stale-usage  Request mode — `core_geoblock_plugin_request-mode` does not state default true / fail-open on lookup only
- [x] language-gap  Plugin root — `core_geoblock_plugin_packages` does not reject the on-disk unpack path
