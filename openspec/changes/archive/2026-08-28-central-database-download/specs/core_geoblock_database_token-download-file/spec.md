## REMOVED Requirements

### Requirement: Token download uses the configured code exactly
**Reason**: The plugin no longer builds IP2Location `file=` URLs from `databaseAutoUpdateCode`.
**Migration**: Put the official URL on a catalog entry (`?token=…&file=DB8BINIPV6`), set `databaseType` = `bin`, `archive` = `zip`, and point `ip2location_download_geo` (or `ip2location_download_asn`) at that key.

### Requirement: Empty token keeps the free LITE URL
**Reason**: Empty download pointer means no HTTP fetch. The hardcoded lite CDN URL is removed.
**Migration**: Catalog entry `url` = `https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP`, `databaseType` = `bin`, `archive` = `zip`. Set `ip2location_download_geo` and `databaseAutoUpdateDir`.

### Requirement: On-disk names use the configured code
**Reason**: Dated files are named by catalog key (`YYYYMMDD_<key>`), not by package code.
**Migration**: Re-download into `databaseAutoUpdateDir`. Old `*IP2LOCATION-LITE-{code}.IPV6.BIN` and `YYYYMMDD_geo` names are not selected.
