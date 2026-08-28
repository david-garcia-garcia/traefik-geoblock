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
Traefik Config key that names the implementation. Empty defaults to `ip2location`. Implemented: `ip2location`, `ipinfo`, `maxmind`. Unknown values fail `New`.

## Overview

`New` calls `openDatabaseProvider`. Request path calls `Lookup` and gets a `Record`. `requestHeaderEnrich` writes every mapped header; an empty field is the string `null`. Country on a private IP is `PRIVATE`. IP2Location LITE DB1 is country-only. Region/city/ISP/domain need DB8 or richer. ASN comes from a second ASN LITE BIN. IPinfo Lite is one MMDB (`ipinfo_lite.mmdb`) with country + ASN; region/city stay empty on the Record. MaxMind / GeoLite2 Country and City use nested `country.iso_code`. The bundled seed is official dummy `GeoIP2-Country-Test.mmdb` (not a live GeoLite file).

## How to use

- Set `databaseProvider` (`ip2location` default, `ipinfo`, or `maxmind`).
- Call `Lookup(ip)` from the plugin. Use `Record.Country` for allow/block. Map errors through `banIfError`.
- Map headers with `requestHeaderEnrich`. Unknown keys fail `New`. Write `null` when the Record field is empty.
- Pass the plugin `New` context into the vendor constructor (`openDatabaseProvider`). Do not pass `req.Context()`.
- File location and keep-current: see Source (`core_geoblock_database_source`). Open and hot-swap: see Wrapper (`core_geoblock_database_wrapper`).
- IPinfo maps `country_code` onto Country, plus `country_name`, continent, `isp` (`as_name`), `domain` (`as_domain`), `asn`. Region/city stay empty on Lite.
- MaxMind maps nested `country.iso_code`. ASN/ISP stay empty on Country/City files. The plugin does not parse `accountId:licenseKey`.
- `countryHeader` is deprecated. `New` copies it onto `requestHeaderEnrich` as key `country` when that header name is unset. Prefer `requestHeaderEnrich` only.

## Pattern snippet

```go
rec, err := p.Lookup(ip)
req.Header.Set("X-Geo-Country", rec.Field("country"))
```

## Key files

- `pkg/dbprovider` — `Provider`, `Record`, meta keys
- `pkg/ip2location` — geo Lookup plus optional ASN
- `pkg/ipinfo` — IPinfo Lite/Core/Plus field mapping
- `pkg/maxmind` — GeoIP2 / GeoLite2 field mapping
- `plugin.go` — `requestHeaderEnrich`, `applyGeoHeaders`
