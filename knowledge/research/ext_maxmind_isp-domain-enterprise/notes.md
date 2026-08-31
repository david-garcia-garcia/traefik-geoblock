# GeoIP2 ISP, Domain, and Enterprise fields

Official MaxMind GeoIP2 **ISP**, **Domain**, and **Enterprise** binary (MMDB) field paths, and which of them map to this plugin’s `Record` keys. Country / City / ASN-only GeoLite2 paths stay in [ext_maxmind_geolite2-database](../ext_maxmind_geolite2-database/). Not how this plugin should wrap a provider.

Local `Record` keys used only for the mapping section: `country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn` (`pkg/dbprovider/provider.go`).

ISP and Domain are **standalone** paid MMDB products (no country/city). Enterprise is one nested record that includes City-like geolocation **plus** ISP, domain, and ASN under `traits`.

## GeoIP2 ISP (flat)

Owner: [GeoIP ISP binary database fields](https://dev.maxmind.com/geoip/docs/databases/isp/binary/). Each network is a flat map. Empty keys are omitted (same MaxMind MMDB convention as Country).

| Field | Type | Official meaning |
| --- | --- | --- |
| `autonomous_system_number` | uint32 | ASN associated with the IP |
| `autonomous_system_organization` | string | Organization for that registered ASN |
| `isp` | string | Name of the ISP associated with the IP |
| `mobile_country_code` | string | MCC for the IP and ISP |
| `mobile_network_code` | string | MNC for the IP and ISP |
| `organization` | string | Organization associated with the IP |

No `country`, `continent`, `subdivisions`, `city`, or `domain` keys. Product page ([GeoIP ISP Databases](https://dev.maxmind.com/geoip/docs/databases/isp/)): “Determine the Internet Service Provider, organization name, and autonomous system number and organization.” CSV zip `GeoIP2-ISP-CSV_{YYYYMMDD}.zip`. Dummy fixture: `GeoIP2-ISP-Test.mmdb` on maxmind/MaxMind-DB.

oschwald `geoip2-golang` v1.13.0 `ISP` struct tags match those six names (flat). `DatabaseType` values `GeoIP2-ISP` and `GeoIP2-Precision-ISP`.

Extracts: [.sources/isp-binary.md](.sources/isp-binary.md), [.sources/isp.md](.sources/isp.md), [.sources/geoip2-golang-v1.13.0-reader.go.md](.sources/geoip2-golang-v1.13.0-reader.go.md)

## GeoIP2 Domain (flat)

Owner: [GeoIP Domain binary database fields](https://dev.maxmind.com/geoip/docs/databases/domain/binary/). One field:

| Field | Type | Official meaning |
| --- | --- | --- |
| `domain` | string | Second-level domain for the IP (`example.com` or `example.co.uk`, **not** `foo.example.com`) |

No geo, ISP, or ASN keys. Product page ([GeoIP Domain Databases](https://dev.maxmind.com/geoip/docs/databases/domain/)): “Look up the second-level domain names associated with IPv4 and IPv6 addresses.” CSV zip `GeoIP2-Domain-CSV_{YYYYMMDD}.zip`. Dummy fixture: `GeoIP2-Domain-Test.mmdb`.

`geoip2-golang` v1.13.0 `Domain` struct: `` Domain `maxminddb:"domain"` ``. `DatabaseType` `GeoIP2-Domain`. Test lookup `1.2.0.0` → `maxmind.com`.

This is **not** the same meaning as IPinfo `as_domain` (ASN organization website). See [ext_ipinfo_core-plus-fields](../ext_ipinfo_core-plus-fields/).

Extracts: [.sources/domain-binary.md](.sources/domain-binary.md), [.sources/domain.md](.sources/domain.md)

## GeoIP2 Enterprise (nested)

Owner: [GeoIP Enterprise binary database fields](https://dev.maxmind.com/geoip/docs/databases/enterprise/binary/). Top-level record is a **map**. Keys that map to empty/undefined values are omitted. Top-level keys: `city`, `continent`, `country`, `location`, `postal`, `registered_country`, `represented_country`, `subdivisions` (array, **largest to smallest**), `traits`.

Geo paths that matter for `Record` (same nest as GeoIP2 City, plus confidence on several maps):

| Path | Type | Official meaning |
| --- | --- | --- |
| `country.iso_code` | string | Two-character ISO 3166-1 alpha code for the **located** country |
| `country.names` | map | Locale → localized country name (`en`, `de`, `ja`, …) |
| `continent.code` | string | Two-character continent code (`NA`, `OC`, …) |
| `continent.names` | map | Locale → localized continent name |
| `subdivisions[].iso_code` | string | Up to three characters; subdivision portion of ISO 3166-2 |
| `subdivisions[].names` | map | Locale → localized subdivision name |
| `city.names` | map | Locale → localized city name |

`registered_country.iso_code` is where the ISP **registered** the block and may differ from located `country`. MaxMind tells CSV users not to key on `*_name` fields; recommended country key is `country_iso_code` (binary equivalent: `country.iso_code`).

`traits` (Enterprise-only extras versus GeoIP2 City):

| Path | Type | Official meaning |
| --- | --- | --- |
| `traits.isp` | string | ISP name |
| `traits.domain` | string | Second-level domain (same definition as the Domain product) |
| `traits.autonomous_system_number` | uint32 | ASN |
| `traits.autonomous_system_organization` | string | ASN organization |
| `traits.organization` | string | Organization associated with the IP |
| `traits.connection_type` | string | `Cable/DSL`, `Cellular`, `Corporate`, or `Satellite` (more values may be added) |
| `traits.user_type` | string | Closed list (`business`, `residential`, `hosting`, …) |
| `traits.mobile_country_code` / `traits.mobile_network_code` | string | MCC / MNC |
| `traits.is_anycast` | boolean | Present only when true |

Product page ([GeoIP Enterprise Databases](https://dev.maxmind.com/geoip/docs/databases/enterprise/)): geolocation (country, region, state, city, postal) plus confidence, ISP, domain, and connection type. Dummy fixture: `GeoIP2-Enterprise-Test.mmdb`. CSV zip `GeoIP2-Enterprise-CSV_{YYYYMMDD}.zip`. Continent codes listed on the CSV locations table: `AF`, `AN`, `AS`, `EU`, `NA`, `OC`, `SA`.

`geoip2-golang` v1.13.0 `Enterprise` struct tags match those nests (`country` / `iso_code` / `names`, `continent` / `code`, `subdivisions` / `iso_code`, `city` / `names`, `traits` / `isp` / `domain` / `autonomous_system_number`). `DatabaseType` `GeoIP2-Enterprise`. That unofficial library also tags older/extra trait keys (`static_ip_score`, `is_anonymous_proxy`, `is_legitimate_proxy`, `is_satellite_provider`) that the **current** official binary traits table does not list. Follow official for the field list; follow source for what that tagged struct decodes.

Extracts: [.sources/enterprise-binary.md](.sources/enterprise-binary.md), [.sources/enterprise.md](.sources/enterprise.md), [.sources/geoip2-golang-v1.13.0-reader.go.md](.sources/geoip2-golang-v1.13.0-reader.go.md)

## Mapping to plugin `Record`

Authority: **inference** from official schema + `pkg/dbprovider/provider.go`. MaxMind does not mention this plugin. Same ISO-in-`Country` convention as the GeoLite2 finding.

| `Record` key | GeoIP2 ISP | GeoIP2 Domain | GeoIP2 Enterprise |
| --- | --- | --- | --- |
| `country` | *(none)* | *(none)* | `country.iso_code` |
| `country_name` | *(none)* | *(none)* | `country.names` (locale; MaxMind warns not to use names as keys) |
| `continent` | *(none)* | *(none)* | `continent.names` (locale) |
| `continent_code` | *(none)* | *(none)* | `continent.code` |
| `region` | *(none)* | *(none)* | first `subdivisions[]` (largest): `iso_code` or `names`. Official order is largest → smallest. GeoLite2 finding used `iso_code`. |
| `city` | *(none)* | *(none)* | `city.names` (locale) |
| `isp` | `isp` | *(none)* | `traits.isp` |
| `domain` | *(none)* | `domain` | `traits.domain` |
| `asn` | `autonomous_system_number` (uint, no `AS` prefix) | *(none)* | `traits.autonomous_system_number` (uint) |

`autonomous_system_organization` / `organization` are not `Record.isp`. City-only GeoIP2/GeoLite2 still has no `traits.isp` / `traits.domain` / ASN (see the GeoLite2 finding).

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| ISP binary fields (`isp`, ASN, org, MCC/MNC) | https://dev.maxmind.com/geoip/docs/databases/isp/binary/ | official |
| Domain binary field `domain` (second-level) | https://dev.maxmind.com/geoip/docs/databases/domain/binary/ | official |
| Enterprise nested geo + `traits.isp` / `traits.domain` / `traits.autonomous_system_number` | https://dev.maxmind.com/geoip/docs/databases/enterprise/binary/ | official |
| Subdivisions ordered largest to smallest | same Enterprise binary page | official |
| Do not key on `*_name`; country key is ISO | https://dev.maxmind.com/geoip/docs/databases/enterprise/ (CSV “Returned Values as … Keys”) | official |
| Continent code list `AF AN AS EU NA OC SA` | same Enterprise CSV locations table | official |
| geoip2-golang tags match official paths; extra unofficial trait tags exist | github.com/oschwald/geoip2-golang@v1.13.0 `reader.go` | source (unofficial API) |
| Map paths → plugin `Record` | inferred from official schema + `pkg/dbprovider/provider.go` | inference |

Conflicts:

1. **geoip2-golang extra `traits` tags vs official Enterprise binary table** — unofficial struct includes `static_ip_score`, `is_anonymous_proxy`, `is_legitimate_proxy`, `is_satellite_provider`. Official binary traits table does not list those (CSV marks anonymous-proxy / satellite-provider deprecated). Follow official for “what Enterprise documents”; source for “what this tagged struct will decode if present.”
2. **IPinfo `as_domain` vs MaxMind `domain`** — different products, different meanings (ASN org website vs second-level domain of the IP). Do not treat them as the same column.

## References

- https://dev.maxmind.com/geoip/docs/databases/isp/binary/
- https://dev.maxmind.com/geoip/docs/databases/isp/
- https://dev.maxmind.com/geoip/docs/databases/domain/binary/
- https://dev.maxmind.com/geoip/docs/databases/domain/
- https://dev.maxmind.com/geoip/docs/databases/enterprise/binary/
- https://dev.maxmind.com/geoip/docs/databases/enterprise/
- https://github.com/oschwald/geoip2-golang/blob/v1.13.0/reader.go
- `pkg/dbprovider/provider.go`
- [ext_maxmind_geolite2-database](../ext_maxmind_geolite2-database/)
- [ext_ipinfo_core-plus-fields](../ext_ipinfo_core-plus-fields/)
