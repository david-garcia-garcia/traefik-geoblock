---
url: https://github.com/ipinfo/sample-database/blob/ff663e000ab0fe32e28b5911be262a01cf284d9a/IPinfo%20Plus/ipinfo_plus_sample.csv
title: IPinfo Plus sample CSV header and rows
fetched: 2026-08-31
authority: source
ref: github.com/ipinfo/sample-database@ff663e000ab0fe32e28b5911be262a01cf284d9a:IPinfo Plus/ipinfo_plus_sample.csv
---

Header (flat, no `isp`; Core columns plus Plus extras):

`network,city,region,region_code,country,country_code,continent,continent_code,latitude,longitude,timezone,postal_code,dma_code,geoname_id,radius,asn,as_name,as_domain,as_type,carrier_name,mcc,mnc,as_changed,geo_changed,is_anonymous,is_anycast,is_hosting,is_mobile,is_satellite,is_proxy,is_relay,is_tor,is_vpn,privacy_name`

Example row `1.0.0.0/31`: same Sydney / AU / AS13335 Cloudflare values as Core, plus geoname_id=2147714, radius=5000, as_changed=2021-05-01, geo_changed=2026-02-08; dma_code / carrier_name / mcc / mnc / privacy_name empty.

JSON sibling is NDJSON with those flat keys (`dma_code`/`carrier_name`/`mcc`/`mnc`/`privacy_name` null). Row `1.0.0.2` has `is_anonymous=true` and `is_vpn=true`.
