---
url: https://community.ipinfo.io/t/using-ipinfos-mmdb-data-downloads-with-golang/4415
title: Using IPinfo's MMDB data downloads with Golang
fetched: 2026-08-27
authority: vendor
---

IPinfo Community article (Abdullah, 2023-12-12). Says MMDB needs a reader package, not the API package.

Install: `go get github.com/oschwald/maxminddb-golang`. Usage: `maxminddb.Open(path)`, `db.Lookup(ip, &result)` with `net.IP` and struct tags `maxminddb:"..."`. Tags must match the IPinfo database schema (`https://ipinfo.io/developers/database-types`).

Worked example is the privacy database (`hosting`, `proxy`, `tor`, `vpn`, `relay`, `service`), not Lite `country_code`. This is the v1 oschwald API.
