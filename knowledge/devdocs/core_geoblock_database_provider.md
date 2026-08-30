# Database provider

## Language

**Provider**:
The merged Lookup the plugin calls for one IP (`dbprovider.Provider`). Enabled catalog rows fill one `Record`; first non-empty field wins.
_Avoid_: `databaseProvider` as a Traefik config key; a vendor constructor; `vendor` on a catalog row

**Record**:
Country, region, city, ISP, domain, and ASN for one IP. Lookup writes country onto `countryHeader`. The block stage reads that header. Other fields may be empty.

**requestHeaderEnrich**:
Traefik map of request header name → metadata key (`country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn`).

## Overview

`NewCore` opens enabled `databaseSources` only when `mode` is `enrich` or `enrichandblock`. Request path calls `Lookup` and gets a `Record`, then writes `countryHeader` and `requestHeaderEnrich`. An empty field is the string `null`. Country on a private IP is `PRIVATE`. `block` and `disabled` do not open catalog sources. A `bin` LITE DB1 is country-only. Region/city/ISP/domain need a richer BIN. ASN LITE is a second `bin` row with `fields: [asn]` (`Get_asn`; no shipped seed). An `mmdb` row fills the same Record columns from flat and nested tags. The bundled seeds are `ipinfo_lite.mmdb` and official dummy `GeoIP2-Country-Test.mmdb`. Shipped `default_geolite` is a disabled unofficial Country GET.

## How to use

- Enable catalog rows (`databaseType`, optional `defaultFile`, optional `fields`). Empty `fields` keeps every mapped key. `fields` is normalized Record keys, not raw DB column names. Omitted `enabled` is on. Disable `default_ip2location` when another country source should be the only one.
- Call `Lookup(ip)` from the plugin only in lookup modes. Write `Record.Country` to `countryHeader`. The block stage reads `countryHeader`. Map errors through `banIfError`.
- Map extra headers with `requestHeaderEnrich`. Unknown keys fail `New`. Write `null` when the Record field is empty. A `country` mapping must use the same header as `countryHeader`.
- Pass the plugin `New` context into wrapper open (`openCatalogSources`). Do not pass `req.Context()`.
- File location and keep-current: see Source (`core_geoblock_database_source`). Open and field maps: see Wrapper (`core_geoblock_database_wrapper`).
- MMDB Lookup fills Country from `country_code` or nested `country.iso_code`, plus the other normalized keys the file has. The plugin does not parse `accountId:licenseKey`.
- `countryHeader` defaults to `X-IPCountry` when omitted. Lookup writes it; block reads it.

## Pattern snippet

```go
rec, err := p.Lookup(ip)
req.Header.Set("X-Geo-Country", rec.Field("country"))
```

## Key files

- `pkg/dbprovider` — `Provider`, `Combined`, `Record`, meta keys
- `pkg/dbwrappers` — `BIN`, `MMDB`, `LookupRecord`
- `pkg/geoblock/plugin.go` — `openCatalogSources`, `requestHeaderEnrich`, enrich headers
