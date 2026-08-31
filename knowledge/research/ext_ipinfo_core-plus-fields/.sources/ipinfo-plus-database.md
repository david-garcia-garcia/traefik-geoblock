---
url: https://ipinfo.io/developers/ipinfo-plus-database
title: IPinfo Plus Database
fetched: 2026-08-31
authority: official
---

Plus is an enterprise-grade database: location, insights and confidence, ASN, privacy, carrier, and network flags. Downloads are a separate enterprise offering and are not included in IPinfo Plus API self-serve plans.

Schema includes every Core geo/ASN/flag field, plus: `dma_code`, `geoname_id` (typed TEXT here), `radius` (INTEGER, km), `carrier_name`, `mcc`, `mnc` (typed TEXT here), `as_changed` / `geo_changed` (DATE `YYYY-MM-DD`), `is_proxy`, `is_relay`, `is_tor`, `is_vpn`, `privacy_name` (example NordVPN).

`as_name` = “Name of the ASN organization” (example British Telecommunications PLC). `as_domain` = “Organization domain name of the ASN” (example bt.com). `asn` example `AS2856`. `city`/`region`/`region_code`/`country`/`country_code` examples: Weymouth / England / ENG / United Kingdom / GB.

No `isp` column.

Download slugs (token required, `curl -L`):

```
https://ipinfo.io/data/ipinfo_plus.csv.gz?token=$TOKEN
https://ipinfo.io/data/ipinfo_plus.mmdb?token=$TOKEN
https://ipinfo.io/data/ipinfo_plus.json.gz?token=$TOKEN
https://ipinfo.io/data/ipinfo_plus.parquet?token=$TOKEN
```

File metadata on fetch day: Last Updated Aug 29, 2026; Lines 442,374,096. MMDB `ipinfo_plus.mmdb` 3.36 GB. Links sample-database GitHub repo.
