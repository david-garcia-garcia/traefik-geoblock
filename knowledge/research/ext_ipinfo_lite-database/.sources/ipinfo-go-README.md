---
url: https://github.com/ipinfo/go/blob/7483de07572f709126bc2ff0158afaf3c15af9b0/README.md
title: IPinfo Go Client Library README
fetched: 2026-08-27
authority: official
ref: github.com/ipinfo/go@7483de07572f709126bc2ff0158afaf3c15af9b0:README.md
---

Official Go client for the IPinfo.io IP address API. Not described as a local database reader.

Install: `go get github.com/ipinfo/go/v2/ipinfo`. Token from signup; token page `https://ipinfo.io/account/token`. `ipinfo.NewClient(nil, nil, token)` then `GetIPInfo`.

README free-plan note: 50,000 requests per month (classic/Core API). Separately documents Lite API: `NewLiteClient` / `GetIPInfoLite`; “authentication with your token is still required.” Lite example prints ASN, Country (name), CountryCode.

No MMDB or local-file lookup instructions in this README.
