---
url: https://github.com/ipinfo/sample-database/blob/ff663e000ab0fe32e28b5911be262a01cf284d9a/IPinfo%20Lite/README.md
title: IPinfo Lite sample folder README
fetched: 2026-08-27
authority: official
ref: github.com/ipinfo/sample-database@ff663e000ab0fe32e28b5911be262a01cf284d9a:IPinfo Lite/README.md
---

Lite is the free download: country + continent + ASN, IPv4 and IPv6 in one file, updated daily, CC-BY-SA 4.0. Sample files: ipinfo_lite_sample.csv, .json, .mmdb, .parquet.

Schema table identical to the official Lite database page. Alternate `country_asn` schema uses start_ip/end_ip; `country` there is the ISO code.

Full-file curl still uses `?token=$YOUR_TOKEN` and `ipinfo_lite.{csv.gz,mmdb,json.gz,parquet}`. Samples in this folder are limited to 100 rows.
