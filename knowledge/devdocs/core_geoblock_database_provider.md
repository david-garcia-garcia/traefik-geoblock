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
- Download is a named catalog (`databaseDownloads`) plus pointers (`ip2location_download_geo`, `ip2location_download_asn`, `ipinfo_download`, `maxmind_download`) and shared `databaseAutoUpdateDir`. Each catalog value is `url`, `databaseType` (`bin`/`mmdb`), `archive` (`none`/`zip`/`tar.gz`), optional `headers`. Empty pointer = seed/path only. A pointer to a missing key, unknown type/archive, or a bound URL without the dir fails `New`. Unused pointers for another provider are ignored.
- At init the auto-update dir is preferred (newest `YYYYMMDD_<catalogKey>`). `ip2location_databaseFilePath` / `ip2location_asnDatabaseFilePath` are the seed when that dir has no file for the bound key.
- IPinfo: empty `ipinfo_databaseFilePath` uses bundled `ipinfo_lite.mmdb`. Point `ipinfo_download` at a catalog entry whose `url` is `https://ipinfo.io/data/ipinfo_{lite|core|plus}.mmdb?token=`, `databaseType` `mmdb`, `archive` `none`. Country is `country_code`. Also fills `country_name`, `continent`, `continent_code`, `region`, `city` (empty on Lite), `isp` (`as_name`), `domain` (`as_domain`), `asn`.
- MaxMind: empty `maxmind_databaseFilePath` uses bundled `GeoIP2-Country-Test.mmdb`. Point `maxmind_download` at a catalog entry with the official permalink, `databaseType` `mmdb`, `archive` `tar.gz`, and `Authorization: Basic …` on `headers`. The plugin does not parse `accountId:licenseKey`. Country is nested `country.iso_code`. ASN/ISP stay empty on Country/City files.
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
- `pkg/ipinfo` — IPinfo Lite MMDB Lookup + hot-swap
- `pkg/maxmind` — GeoIP2 / GeoLite2 MMDB Lookup + hot-swap
- `pkg/dbdownload` — shared GET, unpack-by-`archive`, date-by-`databaseType`, dated write, ticker
- `pkg/dbutils` — HTTP GET, dated-file find, `DownloadHint`
- `plugin.go` — `requestHeaderEnrich`, `applyGeoHeaders`
