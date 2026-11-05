# 2026-11-03-plugin-path-glob

issueHost: local
issueRef: none

## Asks

1. Find the plugin folder without a fixed path. Accept a glob in TRAEFIK_PLUGIN_GEOBLOCK_PATH (example: /plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock). If the variable is unset, try by default that glob and /plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock. If several matches, keep the one that contains seeds/<defaultFile>. filepath.Glob is Go stdlib and should work under Traefik Yaegi, but it must be tested. Compatible with current configurations (an exact directory path must still work).
2. If there is no database file but the source has a URL, wait for the download instead of failing startup. Start without a database with a WARN and let auto-update download it. Meanwhile lookups return an error (pkg/dbwrappers/bin.go around the nil-handle path), and request fate depends on banIfError: true blocks, false passes unfiltered. That false path is a security decision and must be decided and documented. Update openspec/specs/core_geoblock_database_url-download, which today requires startup to fail.
   Additional, same change: if defaultFile is set but that file cannot be located, AND the source has a URL, log a WARN and continue. Do not refuse to start. The URL is how a file is obtained later. Today a non-empty missing defaultFile sets AllowMissing false and New fails before the updater starts (pkg/geoblock/plugin.go AllowMissing; pkg/dbwrappers/lifecycle.go initialize; pkg/fileutils/fileutils.go Search). A missing defaultFile with NO url stays a startup failure. This applies to every source type that can have a URL, not only BIN.
3. Lower the BundledFile error to DEBUG or WARN when a downloaded file already exists.
4. Update the README with Traefik's new plugin folder (gop-* under /plugins-storage/sources).
