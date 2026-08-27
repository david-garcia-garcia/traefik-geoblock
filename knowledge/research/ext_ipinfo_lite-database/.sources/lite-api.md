---
url: https://ipinfo.io/developers/lite-api
title: IPinfo Lite API
fetched: 2026-08-27
authority: official
---

Free-tier API for every IPinfo user. Unlimited country-level geo and basic ASN. No daily or monthly limit. Based on the IPinfo Lite database.

Token required on the URL: `curl https://api.ipinfo.io/lite/8.8.8.8?token=$TOKEN` and `/lite/me`.

Response fields: ip, asn, as_name, as_domain, country_code, country, continent_code, continent. Same shape as the Lite database (plus ip).

Also documents v4/v6 API hosts and a batch endpoint (`/batch/lite`) up to 1,000 IPs per call.
