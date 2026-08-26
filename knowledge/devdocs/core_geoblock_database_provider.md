# Database provider

## Language

**DatabaseProvider**:
The interface the plugin uses to open a geo database, look up metadata, and run auto-update.
_Avoid_: calling an IP2Location SDK type from `plugin.go`.

**Record**:
Country, region, city, ISP, domain, and ASN for one IP. Country is used for allow/block. Other fields may be empty.

**requestHeaderEnrich**:
Traefik map of request header name → metadata key (`country`, `region`, `city`, `isp`, `domain`, `asn`).

**databaseProvider**:
Traefik Config key that names the implementation. Empty defaults to `ip2location`. Unknown values fail `New`.

## Overview

`New` calls `openDatabaseProvider`. Request path calls `Lookup` and gets a `Record`. `requestHeaderEnrich` writes keys that have a value. IP2Location LITE DB1 is country-only. Region/city/ISP/domain need DB8 or richer. ASN comes from a second ASN LITE BIN.

## How to use

- Set `databaseProvider` (or leave empty for `ip2location`).
- Call `Lookup(ip)` from the plugin. Use `Record.Country` for allow/block. Map errors through `banIfError`.
- Map headers with `requestHeaderEnrich`. Unknown keys fail `New`. Do not write empty or “unavailable” BIN fields.
- ASN auto-update is opt-in (`ip2location_asnDatabaseAutoUpdate`). It reuses the geo dir and token. Default package code is `DBASNLITEBINIPV6`.
- Keep `countryHeader` as the single-header country alias.

## Pattern snippet

```go
rec, err := p.Lookup(ip)
req.Header.Set("X-Geo-Country", rec.Field("country"))
```

## Key files

- `pkg/dbprovider` — `Provider`, `Record`, meta keys
- `pkg/ip2location` — geo `Get_all` plus optional ASN `Get_asn`
- `plugin.go` — `requestHeaderEnrich`, `applyGeoHeaders`
