# Database provider

## Language

**DatabaseProvider**:
The interface the plugin uses to open a geo database, look up metadata, and run auto-update.
_Avoid_: calling an IP2Location SDK type from `plugin.go`.

**Record**:
Country, region, city, ISP, domain, and ASN for one IP. Country is used for allow/block. Other fields may be empty.

**requestHeaderEnrich**:
Traefik map of request header name → metadata key (`country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn`).

**databaseProvider**:
Traefik Config key that names the implementation. Empty defaults to `ip2location`. Implemented: `ip2location`, `ipinfo`. Unknown values fail `New`.

## Overview

`New` calls `openDatabaseProvider`. Request path calls `Lookup` and gets a `Record`. `requestHeaderEnrich` writes every mapped header; an empty field is the string `null`. Country on a private IP is `PRIVATE`. IP2Location LITE DB1 is country-only. Region/city/ISP/domain need DB8 or richer. ASN comes from a second ASN LITE BIN. IPinfo Lite is one MMDB (`ipinfo_lite.mmdb`) with country + ASN; region/city stay empty on the Record.

## How to use

- Set `databaseProvider` (`ip2location` default, or `ipinfo`).
- Call `Lookup(ip)` from the plugin. Use `Record.Country` for allow/block. Map errors through `banIfError`.
- Map headers with `requestHeaderEnrich`. Unknown keys fail `New`. Write `null` when the Record field is empty.
- ASN auto-update is opt-in (`ip2location_asnDatabaseAutoUpdate`) and only downloads when `ip2location_databaseAutoUpdateToken` is set. The public lite CDN does not host the ASN BIN. Default package code is `DBASNLITEBINIPV6`.
- At init the auto-update dir is preferred. `ip2location_databaseFilePath` / `ip2location_asnDatabaseFilePath` are the seed when that dir has no BIN.
- IPinfo: `ipinfo_databaseAutoUpdateCode` is `lite` (default), `core`, or `plus`. Seed and download file are `ipinfo_{code}.mmdb`. Empty `ipinfo_databaseFilePath` uses bundled `ipinfo_lite.mmdb` when code is lite. Auto-update downloads `https://ipinfo.io/data/ipinfo_{code}.mmdb?token=` only when `ipinfo_databaseAutoUpdateToken` is set. Country is `country_code`. Also fills `country_name`, `continent`, `continent_code`, `region`, `city` (empty on Lite), `isp` (`as_name`), `domain` (`as_domain`), `asn`.
- `countryHeader` is deprecated. `New` copies it onto `requestHeaderEnrich` as key `country` when that header name is unset. Prefer `requestHeaderEnrich` only.

## Gotchas

- **Do** open the Lite MMDB with `os.ReadFile` + `maxminddb.FromBytes`. After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1` so Yaegi never loads upstream mmap / `x/sys` (`incomplete type ifreq`).

## Pattern snippet

```go
rec, err := p.Lookup(ip)
req.Header.Set("X-Geo-Country", rec.Field("country"))
```

## Key files

- `pkg/dbprovider` — `Provider`, `Record`, meta keys
- `pkg/ip2location` — geo `Get_all` plus optional ASN `Get_asn`
- `pkg/ipinfo` — IPinfo Lite MMDB + token auto-update
- `plugin.go` — `requestHeaderEnrich`, `applyGeoHeaders`
