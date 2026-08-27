---
url: https://github.com/maxmind/geoipupdate/blob/main/doc/GeoIP.conf.md
title: GeoIP.conf — configuration file for geoipupdate
fetched: 2026-08-27
authority: official
ref: github.com/maxmind/geoipupdate:doc/GeoIP.conf.md (main)
---

Required: `AccountID` (formerly `UserId`), `LicenseKey` (case-sensitive), `EditionIDs` (space-separated edition IDs; letters, digits, dashes; formerly `ProductIds`). Example edition: `GeoIP2-City` downloads the GeoIP City database.

Default download host: `https://updates.maxmind.com`. CSV databases are not supported by geoipupdate (see program README).
