# IPinfo Core and Plus fields

Official IPinfo Core and Plus downloadable-database columns, how they differ from Lite, and which names map to this plugin’s `Record` keys. Not how this plugin should wrap a provider.

Local `Record` keys used only for the mapping section: `country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn` (`pkg/dbprovider/provider.go`). Lite-only schema and download facts stay in [ext_ipinfo_lite-database](../ext_ipinfo_lite-database/).

## Lite vs Core vs Plus

IPinfo publishes three **bundle** databases on [IP Database Types](https://ipinfo.io/developers/database-types):

| Product | Official one-line | Paid download? |
| --- | --- | --- |
| IPinfo Lite | Country + ASN. CC-BY-SA 4.0 | Free with account token ([Lite database](https://ipinfo.io/developers/ipinfo-lite-database)) |
| IPinfo Core | Location + ASN + network flags (VPN/anycast/carrier/satellite/hosting) | Enterprise download; **not** included in Core API self-serve ([Core database](https://ipinfo.io/developers/ipinfo-core-database)) |
| IPinfo Plus | Location with stability insights + detailed privacy + ASN + network flags | Same: enterprise download, not Plus API self-serve ([Plus database](https://ipinfo.io/developers/ipinfo-plus-database)) |

Which **named columns** exist is owned by each product page (and the Lite page for Lite). The comparison table on Database Types lists every bundle column; the per-product checkmarks did not survive HTML-to-text fetch. Follow the product schema tables below.

Columns that appear on **Lite, Core, and Plus**: `network`, `country` (name), `country_code`, `continent`, `continent_code`, `asn`, `as_name`, `as_domain`.

Columns on **Core and Plus only** (absent from Lite): `city`, `region`, `region_code`, `latitude`, `longitude`, `timezone`, `postal_code`, `as_type`, `is_anonymous`, `is_anycast`, `is_hosting`, `is_mobile`, `is_satellite`.

Columns on **Plus only** (absent from Core and Lite): `dma_code`, `geoname_id`, `radius`, `carrier_name`, `mcc`, `mnc`, `as_changed`, `geo_changed`, `is_proxy`, `is_relay`, `is_tor`, `is_vpn`, `privacy_name`.

None of Lite, Core, or Plus documents an `isp` column. Closest name fields are `as_name` (ASN organization) and Plus-only `carrier_name` (mobile carrier). `as_type` is a category (`ISP`, `Hosting`, `Education`, `Government`, or `Business` in official prose; sample rows use lowercase `isp` / `hosting`).

`as_domain` is the **ASN organization’s** website, not IPinfo’s separate Hosted Domains product.

Formats on both Core and Plus product pages: CSV (gzip), MMDB, JSON (gzip / NDJSON), Parquet. Current download slugs:

```
https://ipinfo.io/data/ipinfo_core.{csv.gz|mmdb|json.gz|parquet}?token=$TOKEN
https://ipinfo.io/data/ipinfo_plus.{csv.gz|mmdb|json.gz|parquet}?token=$TOKEN
```

Extracts: [.sources/database-types.md](.sources/database-types.md), [.sources/ipinfo-core-database.md](.sources/ipinfo-core-database.md), [.sources/ipinfo-plus-database.md](.sources/ipinfo-plus-database.md)

## Official Core fields

Owner: [IPinfo Core Database](https://ipinfo.io/developers/ipinfo-core-database).

| Field | Example | Type | Official meaning |
| --- | --- | --- | --- |
| `network` | `66.202.64.131` | TEXT | CIDR / range / single IP |
| `city` | `Chicago` | TEXT | City |
| `region` | `Illinois` | TEXT | Region / state **name** |
| `region_code` | `IL` | TEXT | Region code, two-letter ISO 3166 |
| `country` | `United States` | TEXT | Country **name** |
| `country_code` | `US` | TEXT | ISO 3166 country code |
| `continent` | `North America` | TEXT | Continent name |
| `continent_code` | `NA` | TEXT | Two-letter continent code |
| `latitude` / `longitude` | `41.85003` / `-87.65005` | FLOAT | Coordinates |
| `timezone` | `America/Chicago` | TEXT | Local timezone |
| `postal_code` | `60666` | TEXT | Postal / zip |
| `asn` | `AS7029` | TEXT | ASN, including the `AS` prefix in examples |
| `as_name` | `Windstream Communications LLC` | TEXT | Name of the ASN organization |
| `as_domain` | `windstream.com` | TEXT | Organization domain name of the ASN |
| `as_type` | `isp` | TEXT | ASN type (ISP / Hosting / Education / Government / Business) |
| `is_anonymous` | `false` | BOOLEAN | Anonymous IP |
| `is_anycast` | `false` | BOOLEAN | Anycast |
| `is_hosting` | `false` | BOOLEAN | Hosting / cloud / data center |
| `is_mobile` | `false` | BOOLEAN | Mobile network |
| `is_satellite` | `false` | BOOLEAN | Satellite internet |

File metadata on fetch day (2026-08-31): Last Updated Aug 29, 2026; `ipinfo_core.mmdb` 666.57 MB.

Extract: [.sources/ipinfo-core-database.md](.sources/ipinfo-core-database.md)

## Official Plus fields

Owner: [IPinfo Plus Database](https://ipinfo.io/developers/ipinfo-plus-database). Super-set of Core, plus:

| Field | Example | Type | Official meaning |
| --- | --- | --- | --- |
| `dma_code` | `13w` | TEXT | Direct Marketing Area id |
| `geoname_id` | `2634202` | TEXT (page) / INTEGER (Database Types) | geonames.org id |
| `radius` | `20` | INTEGER | Location accuracy radius, kilometers |
| `carrier_name` | *(empty in examples)* | TEXT | Mobile carrier organization name |
| `mcc` / `mnc` | *(empty)* | TEXT on Plus page; INTEGER on Database Types | Mobile Country / Network Code |
| `as_changed` | `2025-01-10` | DATE | Last ASN change, `YYYY-MM-DD` |
| `geo_changed` | `2024-11-10` | DATE | Last location change, `YYYY-MM-DD` |
| `is_proxy` | `false` | BOOLEAN | Open web proxy |
| `is_relay` | `false` | BOOLEAN | Location-preserving relay (e.g. iCloud Private Relay) |
| `is_tor` | `false` | BOOLEAN | Tor exit node |
| `is_vpn` | `false` | BOOLEAN | VPN exit node |
| `privacy_name` | `NordVPN` | TEXT | Privacy-service provider name |

Plus page examples reuse the Core geo/ASN columns (`city=Weymouth`, `region=England`, `region_code=ENG`, `country=United Kingdom`, `country_code=GB`, `asn=AS2856`, `as_name=British Telecommunications PLC`, `as_domain=bt.com`). File metadata on fetch day: Last Updated Aug 29, 2026; `ipinfo_plus.mmdb` 3.36 GB.

Extract: [.sources/ipinfo-plus-database.md](.sources/ipinfo-plus-database.md)

## MMDB / sample shape (flat keys)

Official schema tables are **flat** column names. [Filename reference](https://ipinfo.io/developers/database-filename-reference): sample datasets are 100 rows, identical across CSV / JSON / MMDB / Parquet, no token required.

Clone `ipinfo/sample-database@ff663e000ab0fe32e28b5911be262a01cf284d9a`:

- `IPinfo Core/`: `ipinfo_core_sample.{csv,json,mmdb,parquet}`
- `IPinfo Plus/`: `ipinfo_plus_sample.{csv,json,mmdb,parquet}`

Sample CSV/JSON keys match the official tables. Sample values confirm `as_name` is the org name (`Cloudflare, Inc.`) and `as_domain` is the website (`cloudflare.com`). Some rows have null ASN columns. `as_type` appears as `hosting` or `isp`.

Sample MMDBs opened with vendored `github.com/oschwald/maxminddb-golang` v1.13.1 (`Lookup` into `any`):

| | Core sample | Plus sample |
| --- | --- | --- |
| `DatabaseType` | `ipinfo bundle_location_core_sample.mmdb` | `ipinfo bundle_location_plus_sample.mmdb` |
| Payload | **flat** map | **flat** map |
| `1.0.0.1` | `country_code=AU`, `country=Australia`, `continent=Oceania`, `continent_code=OC`, `region=New South Wales`, `region_code=NSW`, `city=Sydney`, `asn=AS13335`, `as_name=Cloudflare, Inc.`, `as_domain=cloudflare.com` | same geo/ASN keys plus `geoname_id`, `radius`, `as_changed`, `geo_changed`, privacy booleans |

`network` decoded empty / absent in the MMDB payload (same as Lite). Empty Plus fields (`dma_code`, `carrier_name`, `mcc`, `mnc`, `privacy_name`) were omitted from the decoded map. There is **no** `isp` key. Tags such as `` `maxminddb:"country_code"` `` match these files. GeoIP2 paths (`country.iso_code`) do **not**.

The sample-repo Core README still shows older download slugs `ipinfo_standard.*` and sample links `ipinfo_standard_sample.*`. On-disk sample files and the official Core page use `ipinfo_core.*`. Follow the official Core page for current slugs.

Extracts: [.sources/database-filename-reference.md](.sources/database-filename-reference.md), [.sources/IPinfo-Core-README.md](.sources/IPinfo-Core-README.md), [.sources/IPinfo-Plus-README.md](.sources/IPinfo-Plus-README.md), [.sources/ipinfo_core_sample.csv.md](.sources/ipinfo_core_sample.csv.md), [.sources/ipinfo_plus_sample.csv.md](.sources/ipinfo_plus_sample.csv.md), [.sources/ipinfo_core_sample.mmdb-probe.md](.sources/ipinfo_core_sample.mmdb-probe.md), [.sources/ipinfo_plus_sample.mmdb-probe.md](.sources/ipinfo_plus_sample.mmdb-probe.md)

## Mapping to plugin `Record`

Authority: **inference** from official schema + `pkg/dbprovider/provider.go`. IPinfo does not mention this plugin. Plugin allow/block already uses ISO alpha-2 in `Country` (same inference as the Lite finding).

| `Record` key | Lite | Core | Plus | Notes |
| --- | --- | --- | --- | --- |
| `country` | `country_code` | `country_code` | `country_code` | ISO. Do not put Lite/Core/Plus `country` (name) here. |
| `country_name` | `country` | `country` | `country` | Full name (`United States`, `Australia`). |
| `continent` | `continent` | `continent` | `continent` | Name (`North America`, `Oceania`). |
| `continent_code` | `continent_code` | `continent_code` | `continent_code` | Two-letter (`NA`, `OC`). |
| `region` | *(none)* | `region` | `region` | Official **name** (`Illinois`, `New South Wales`). Sibling `region_code` is the ISO 3166 region code (`IL`, `NSW`). |
| `city` | *(none)* | `city` | `city` | |
| `isp` | *(none)* | *(none)* | *(none)* | No `isp` column. Closest: `as_name` (ASN org; not documented as ISP). Plus `carrier_name` is mobile carrier only. |
| `domain` | `as_domain` | `as_domain` | `as_domain` | ASN org website, not a hosted-domains list. |
| `asn` | `asn` | `asn` | `asn` | Official examples include the `AS` prefix (`AS13335`). |

Core/Plus extras with no `Record` key: coordinates, timezone, postal, flags, Plus privacy/carrier/stability/dma/geoname/radius.

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Three bundles: Lite / Core / Plus | https://ipinfo.io/developers/database-types | official |
| Core field list + `ipinfo_core.*` slugs | https://ipinfo.io/developers/ipinfo-core-database | official |
| Plus field list + `ipinfo_plus.*` slugs | https://ipinfo.io/developers/ipinfo-plus-database | official |
| Lite fields (no city/region) | https://ipinfo.io/developers/ipinfo-lite-database | official |
| Core/Plus downloads are enterprise, not API self-serve | Core + Plus product pages | official |
| Samples 100 rows, identical across formats, no token | https://ipinfo.io/developers/database-filename-reference | official |
| Sample files + CSV/JSON keys on disk | ipinfo/sample-database@ff663e00 `IPinfo Core/`, `IPinfo Plus/` | source |
| Core/Plus MMDB records are flat; no `network` / `isp` in payload | sample `ipinfo_*_sample.mmdb` + oschwald v1.13.1 | source |
| Map columns → plugin `Record` | inferred from official schema + `pkg/dbprovider/provider.go` | inference |

Conflicts:

1. **`as_name` / `as_domain` prose** — Database Types and the sample-repo READMEs swap the two descriptions (`as_name` = “organization domain name”, `as_domain` = “name of the ASN organization”). Core and Plus **product** pages assign `as_name` = ASN org name and `as_domain` = org domain. Sample values (`as_name=Cloudflare, Inc.`, `as_domain=cloudflare.com`) match the product pages. Follow Core/Plus product pages.
2. **`ipinfo_standard.*` vs `ipinfo_core.*`** — sample-repo Core README still documents `ipinfo_standard.*`. Official Core page and on-disk sample filenames use `ipinfo_core.*`. Follow the official Core page.
3. **`geoname_id` / `mcc` / `mnc` types** — Plus product page types `geoname_id` as TEXT and `mcc`/`mnc` as TEXT; Database Types types them INTEGER. Same names; type annotation differs. Keep both; names are what MMDB tags use.

## References

- https://ipinfo.io/developers/database-types
- https://ipinfo.io/developers/ipinfo-core-database
- https://ipinfo.io/developers/ipinfo-plus-database
- https://ipinfo.io/developers/ipinfo-lite-database
- https://ipinfo.io/developers/database-filename-reference
- https://github.com/ipinfo/sample-database/blob/ff663e000ab0fe32e28b5911be262a01cf284d9a/IPinfo%20Core/README.md
- https://github.com/ipinfo/sample-database/blob/ff663e000ab0fe32e28b5911be262a01cf284d9a/IPinfo%20Plus/README.md
- `pkg/dbprovider/provider.go`
- [ext_ipinfo_lite-database](../ext_ipinfo_lite-database/)
