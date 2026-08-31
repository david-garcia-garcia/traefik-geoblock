---
url: https://github.com/ipinfo/sample-database/blob/ff663e000ab0fe32e28b5911be262a01cf284d9a/IPinfo%20Core/ipinfo_core_sample.csv
title: IPinfo Core sample CSV header and rows
fetched: 2026-08-31
authority: source
ref: github.com/ipinfo/sample-database@ff663e000ab0fe32e28b5911be262a01cf284d9a:IPinfo Core/ipinfo_core_sample.csv
---

Header (flat, no `isp`):

`network,city,region,region_code,country,country_code,continent,continent_code,latitude,longitude,timezone,postal_code,asn,as_name,as_domain,as_type,is_anonymous,is_anycast,is_hosting,is_mobile,is_satellite`

Example row `1.0.0.0/31`: city=Sydney, region=New South Wales, region_code=NSW, country=Australia, country_code=AU, continent=Oceania, continent_code=OC, asn=AS13335, as_name=`Cloudflare, Inc.`, as_domain=cloudflare.com, as_type=hosting.

JSON sibling `ipinfo_core_sample.json` is NDJSON with the same flat keys. Some later rows have null `asn` / `as_name` / `as_domain` / `as_type`. `as_type=isp` appears (example `1.0.4.0/24`, as_name=`Gtelecom Pty Ltd`).
