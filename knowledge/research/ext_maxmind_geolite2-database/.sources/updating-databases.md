---
url: https://dev.maxmind.com/geoip/updating-databases/
title: Updating GeoIP and GeoLite Databases
fetched: 2026-08-27
authority: official
---

Since January 2024, all database downloads use R2 presigned URLs. HTTP clients must follow redirects. Redirect host (HTTPS): `mm-prod-geoip-databases.a2649acb697e2c09b632799562c076f2.r2.cloudflarestorage.com`.

Two update methods: GeoIP Update program (recommended for binary), or direct download (required for CSV).

GeoIP Update: install from GitHub Releases or Docker `maxmindinc/geoipupdate`. GeoIP.conf fields: `AccountID`, `LicenseKey`, `EditionIDs`. Pre-filled conf from account portal. License keys: https://www.maxmind.com/en/accounts/current/license-key

Direct download permalinks: account portal Download Databases. Basic Authentication with account ID (username) and license key (password). Quote the URL in curl/wget.

Official curl (example edition GeoIP2-City-CSV, zip):

```
curl -O -J -L -u YOUR_ACCOUNT_ID:YOUR_LICENSE_KEY \
'https://download.maxmind.com/geoip/databases/GeoIP2-City-CSV/download?suffix=zip'
```

HEAD on a permalink does not count against the daily download limit. Binary downloads are gzip; CSV are zip. MaxMind may limit downloads in a time window.
