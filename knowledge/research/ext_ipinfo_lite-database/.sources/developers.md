---
url: https://ipinfo.io/developers
title: IPinfo Developer Resource
fetched: 2026-08-27
authority: official
---

Official SDKs include Go (also Python, Node.js, PHP).

API token authenticates requests. Methods: `?token=$TOKEN`, HTTP Basic (`curl -u $TOKEN:`), or `Authorization: Bearer $TOKEN`. Token is on the account dashboard.

IPinfo Lite is the free-tier API: country and ASN, no API request quota restrictions. Host `api.ipinfo.io`, path `/lite`.

Rate-limits section: Lite offers unlimited API access. Paid plans have a monthly request limit (429 when exceeded). No daily/hourly cap and no concurrent-request cap described for paid plans.

Database downloads: CSV, MMDB, JSON, Parquet; see Database Download docs.
