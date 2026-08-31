---
url: https://ipinfo.io/developers/ipinfo-core-database
title: IPinfo Core Database
fetched: 2026-08-31
authority: official
---

Core combines location, ASN, and network flags in one database. Downloads are a separate enterprise offering and are not included in IPinfo Core API self-serve plans.

Schema fields: `network`, `city`, `region`, `region_code`, `country` (name, example United States), `country_code` (ISO, example US), `continent`, `continent_code`, `latitude`, `longitude`, `timezone`, `postal_code`, `asn` (example AS7029), `as_name` (“Name of the ASN organization”, example Windstream Communications LLC), `as_domain` (“Organization domain name of the ASN”, example windstream.com), `as_type` (example isp; types ISP, Hosting, Education, Government or Business), `is_anonymous`, `is_anycast`, `is_hosting`, `is_mobile`, `is_satellite`.

No `isp`, `dma_code`, `geoname_id`, `radius`, carrier, privacy-detail, or change-date columns.

Download slugs (token required, `curl -L`):

```
https://ipinfo.io/data/ipinfo_core.csv.gz?token=$TOKEN
https://ipinfo.io/data/ipinfo_core.mmdb?token=$TOKEN
https://ipinfo.io/data/ipinfo_core.json.gz?token=$TOKEN
https://ipinfo.io/data/ipinfo_core.parquet?token=$TOKEN
```

File metadata on fetch day: Last Updated Aug 29, 2026; Lines 159,897,996. MMDB `ipinfo_core.mmdb` 666.57 MB. Links IPinfo Sample Database Repo on GitHub. Sample downloads: CSV, JSON, MMDB.
