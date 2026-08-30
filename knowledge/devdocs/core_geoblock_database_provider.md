# Database provider

## Language

**DatabaseProvider**:
The interface the plugin uses to open a geo database, look up metadata, and run auto-update.
_Avoid_: calling an IP2Location SDK type from `pkg/geoblock`.

**Record**:
Country, region, city, ISP, domain, and ASN for one IP. Lookup writes country onto `countryHeader`. The block stage reads that header. Other fields may be empty.

**requestHeaderEnrich**:
Traefik map of request header name → metadata key (`country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn`).

**databaseProvider**:
Traefik Config key that names the implementation. Empty defaults to `ip2location`. Implemented: `ip2location`, `ipinfo`, `maxmind`. Unknown values fail `New`.

## Overview

`NewCore` calls `openDatabaseProvider` only when `mode` is `enrich` or `enrichandblock`. Request path calls `Lookup` and gets a `Record`, then writes `countryHeader` and `requestHeaderEnrich`. An empty field is the string `null`. Country on a private IP is `PRIVATE`. `block` and `disabled` do not open a DatabaseProvider. IP2Location LITE DB1 is country-only. Region/city/ISP/domain need DB8 or richer. ASN comes from a second ASN LITE BIN. IPinfo Lite is one MMDB (`ipinfo_lite.mmdb`) with country + ASN; region/city stay empty on the Record. MaxMind / GeoLite2 Country and City use nested `country.iso_code`. The bundled seed is official dummy `GeoIP2-Country-Test.mmdb` (not a live GeoLite file). Empty `maxmind_source` binds reserved `default_geolite` (unofficial Country GET).

## How to use

- Set `databaseProvider` (`ip2location` default, `ipinfo`, or `maxmind`).
- Call `Lookup(ip)` from the plugin only in lookup modes. Write `Record.Country` to `countryHeader`. The block stage reads `countryHeader`. Map errors through `banIfError`.
- Map extra headers with `requestHeaderEnrich`. Unknown keys fail `New`. Write `null` when the Record field is empty. A `country` mapping must use the same header as `countryHeader`.
- Pass the plugin `New` context into the vendor constructor (`openDatabaseProvider`). Do not pass `req.Context()`.
- File location and keep-current: see Source (`core_geoblock_database_source`). Open and hot-swap: see Wrapper (`core_geoblock_database_wrapper`).
- IPinfo maps `country_code` onto Country, plus `country_name`, continent, `isp` (`as_name`), `domain` (`as_domain`), `asn`. Region/city stay empty on Lite.
- MaxMind maps nested `country.iso_code`. ASN/ISP stay empty on Country/City files. The plugin does not parse `accountId:licenseKey`.
- `countryHeader` defaults to `X-IPCountry` when omitted. Lookup writes it; block reads it.

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
- `pkg/geoblock/plugin.go` — `requestHeaderEnrich`, `writeDefaultEnrichHeaders`, `writePublicLookupHeaders`
