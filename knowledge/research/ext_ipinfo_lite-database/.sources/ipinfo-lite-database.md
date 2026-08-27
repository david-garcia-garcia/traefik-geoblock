---
url: https://ipinfo.io/developers/ipinfo-lite-database
title: IPinfo Lite Database
fetched: 2026-08-27
authority: official
---

Lite is IPinfo’s free IP data download: country-level geolocation plus basic ASN (ASN, organization name, domain) in one database.

Schema fields: network, country (name), country_code (ISO 3166 two-letter), continent, continent_code, asn (example AS174), as_name, as_domain. No region or city.

Download examples (token required):

```
curl -L https://ipinfo.io/data/ipinfo_lite.csv.gz?token=$TOKEN -o ipinfo_lite.csv.gz
curl -L https://ipinfo.io/data/ipinfo_lite.mmdb?token=$TOKEN -o ipinfo_lite.mmdb
curl -L https://ipinfo.io/data/ipinfo_lite.json.gz?token=$TOKEN -o ipinfo_lite.json.gz
curl -L https://ipinfo.io/data/ipinfo_lite.parquet?token=$TOKEN -o ipinfo_lite.parquet
```

File metadata on fetch day: Last Updated Aug 26, 2026. Formats CSV / JSON / MMDB / Parquet with those filenames.

License: Creative Commons Attribution-ShareAlike 4.0 International. Credit IPinfo with a link (example: “IP address data powered by IPinfo”).

Also documents older free schemas: country_asn, country, asn under `https://ipinfo.io/data/free/...`. Links IPinfo Sample Database Repo on GitHub. Points to Lite API as the unlimited-request alternative.
