---
url: https://github.com/ipinfo/sample-database/blob/ff663e000ab0fe32e28b5911be262a01cf284d9a/IPinfo%20Lite/ipinfo_lite_sample.csv
title: ipinfo_lite_sample.csv (header and first rows)
fetched: 2026-08-27
authority: source
ref: github.com/ipinfo/sample-database@ff663e000ab0fe32e28b5911be262a01cf284d9a:IPinfo Lite/ipinfo_lite_sample.csv
---

Header:

```
network,country,country_code,continent,continent_code,asn,as_name,as_domain
```

First data row: `1.0.0.0/24,Australia,AU,Oceania,OC,AS13335,"Cloudflare, Inc.",cloudflare.com`

Some later rows leave asn/as_name/as_domain empty. Same field names in the sibling NDJSON sample (`country_code`, not nested MaxMind `country.iso_code`). No region or city columns.

On-disk sibling files at this commit: `ipinfo_lite_sample.mmdb`, `ipinfo_lite_sample.parquet`.
