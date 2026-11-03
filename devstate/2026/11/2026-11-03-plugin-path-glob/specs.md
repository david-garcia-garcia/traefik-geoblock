# Specs
change: plugin-path-glob

## FindSpecHost

- deltaId: plugin-root-search-and-url-wait
  verdict: fold
  spec-id: core_geoblock_database_url-download
  confidence: high
  candidates:
    - core_geoblock_database_url-download
    - core_geoblock_database_source-catalog
    - core_geoblock_database_lookup
    - core_geoblock_database_wrapper-reclaim
    - core_geoblock_plugin_request-mode
    - policy-guards-and-seed-fallback/specs/core_geoblock_database_url-download
  why: live leaf already owns env Search, exact joins, and wait-without-file; glob, URL+missing file, miss levels, and banIfError-while-nil are adjustments of that contract, not a new family. In-flight policy-guards ADDED is unreadable dated fallback, a different job.

- deltaId: source-catalog-defaultFile-join
  verdict: fold
  spec-id: core_geoblock_database_source-catalog
  confidence: high
  candidates:
    - core_geoblock_database_source-catalog
    - core_geoblock_database_url-download
  why: "Shipped seed and download rows" restates `{env}/seeds/<defaultFile>` then `{env}/<defaultFile>`; fold a restatement that points at url-download Search so the two leaves do not disagree. Do not calculate GoPath() here.

- skipped: core_geoblock_database_wrapper-reclaim — AllowMissing nil-handle already promised; MMDB field is symmetry, not a new live promise
- skipped: core_geoblock_plugin_request-mode — banIfError on missing country / empty hops already specified; wait-for-download operator contract belongs in url-download

- fold core_geoblock_database_url-download
- fold core_geoblock_database_source-catalog
