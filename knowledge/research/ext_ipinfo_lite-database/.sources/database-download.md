---
url: https://ipinfo.io/developers/database-download
title: IP Database Download
fetched: 2026-08-27
authority: official
---

Downloads need a subscribed/account database. Two paths: dashboard Data Downloads, or curl.

Pattern: `curl -L https://ipinfo.io/data/{filename}?token=$TOKEN -o {filename}`

`https://ipinfo.io/data/...` returns HTTP 302 to a short-lived signed CDN URL. Clients must follow redirects. CDN hosts: dl.assets.ipinfo.io, dl.ipinfo.io (and storage.googleapis.com behind the latter).

Formats: CSV, MMDB (binary tree, best for single-IP lookups in application code), JSON (NDJSON), Parquet.

MMDB: use IPinfo mmdbctl (`mmdbctl read -f json-pretty 8.8.8.8 location.mmdb`) or “MMDB reader libraries supported by IPinfo” (library names were not present in the fetched page HTML).

Download rate limit: 10 times per day, per unique IP / individual device. Checksums endpoint `/data/.../checksums` does not count toward the cap.
