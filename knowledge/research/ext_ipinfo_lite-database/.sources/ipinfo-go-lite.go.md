---
url: https://github.com/ipinfo/go/blob/7483de07572f709126bc2ff0158afaf3c15af9b0/ipinfo/lite.go
title: ipinfo/lite.go
fetched: 2026-08-27
authority: source
ref: github.com/ipinfo/go@7483de07572f709126bc2ff0158afaf3c15af9b0:ipinfo/lite.go
---

`defaultLiteBaseURL = "https://api.ipinfo.io/lite/"`. `LiteClient` is an HTTP API client (http.Client, BaseURL, Token, optional Cache).

`Lite` JSON fields: ip, asn, as_name, as_domain, country_code, country, continent_code, continent, bogon. Extra in-memory fields (CountryName, flags, currency) are filled from country_code, not from a file.

`GetIPInfo` builds `/lite/{ip}` or `/lite/me` and sends `Authorization: Bearer {token}` when Token is set. No filesystem or MMDB open.
