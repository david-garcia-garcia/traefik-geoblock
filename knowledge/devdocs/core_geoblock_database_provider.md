# Database provider

## Language

**Provider**:
The merged Lookup the plugin calls for one IP (`dbprovider.Provider`). Enabled catalog rows fill one `Record`; first non-empty field wins.
_Avoid_: `databaseProvider` as a Traefik config key; a vendor constructor

**Record**:
Country, region, city, ISP, domain, and ASN for one IP. Lookup writes country onto `countryHeader`. The block stage reads that header. Other fields may be empty.

**requestHeaderEnrich**:
Traefik map of request header name → metadata key (`country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn`).

## Overview

`NewCore` opens enabled `databaseSources` only when `mode` is `enrich` or `enrichandblock`. Request path calls `Lookup` and gets a `Record`, then writes `countryHeader` and `requestHeaderEnrich`. An empty field is the string `null`. Country on a private IP is `PRIVATE`. `block` and `disabled` do not open catalog sources. IP2Location LITE DB1 is country-only. Region/city/ISP/domain need DB8 or richer. ASN is a separate catalog row (`vendor: ip2location-asn`). IPinfo Lite is one MMDB (`ipinfo_lite.mmdb`) with country + ASN; region/city stay empty on the Record. MaxMind / GeoLite2 Country and City use nested `country.iso_code`. The bundled seed is official dummy `GeoIP2-Country-Test.mmdb` (not a live GeoLite file). Shipped `default_geolite` is a disabled unofficial Country GET.

## How to use

- Enable catalog rows (`vendor`, optional `defaultFile`). Omitted `enabled` is on. Disable `default_ip2location` when another country source should be the only one.
- Call `Lookup(ip)` from the plugin only in lookup modes. Write `Record.Country` to `countryHeader`. The block stage reads `countryHeader`. Map errors through `banIfError`.
- Map extra headers with `requestHeaderEnrich`. Unknown keys fail `New`. Write `null` when the Record field is empty. A `country` mapping must use the same header as `countryHeader`.
- Pass the plugin `New` context into wrapper open (`openCatalogSources`). Do not pass `req.Context()`.
- File location and keep-current: see Source (`core_geoblock_database_source`). Open and field maps: see Wrapper (`core_geoblock_database_wrapper`).
- IPinfo maps `country_code` onto Country, plus `country_name`, continent, `isp` (`as_name`), `domain` (`as_domain`), `asn`. Region/city stay empty on Lite.
- MaxMind maps nested `country.iso_code`. ASN/ISP stay empty on Country/City files. The plugin does not parse `accountId:licenseKey`.
- `countryHeader` defaults to `X-IPCountry` when omitted. Lookup writes it; block reads it.

## Pattern snippet

```go
rec, err := p.Lookup(ip)
req.Header.Set("X-Geo-Country", rec.Field("country"))
```

## Key files

- `pkg/dbprovider` — `Provider`, `Combined`, `Record`, meta keys
- `pkg/dbwrappers` — `BINSource`, `ASNSource`, `IPinfo`, `GeoIP2`
- `pkg/geoblock/plugin.go` — `openCatalogSources`, `requestHeaderEnrich`, enrich headers
