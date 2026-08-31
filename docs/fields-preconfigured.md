# `fieldsPreconfigured`

Named vendor column maps for a `databaseSources` row. Set `fieldsPreconfigured` **or** a `fields` map — not both. Format (`databaseType`) must match the preset (`bin` or `mmdb`).

Record keys you can map in `requestHeaderEnrich`:

`country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn`

Extra vendor columns (coordinates, postal code, timezone, privacy flags) stay unused. An empty Record field is written as the string `null`.

`fields` is the same map written by hand (`country.iso_code: country`, or `{ key: asn, type: uint32 }`).

See [README.md](../README.md) for reserved catalog keys, download URLs, and which file is used.

## IP2Location (`bin`)

IPv6 BIN uses one token URL and `archive: zip`:

`https://www.ip2location.com/download?token=YOUR_TOKEN&file=`

Put the package you own in `file=`. Codes are official download-client names (`DB8BINIPV6`, `DB1LITEBINIPV6`, …), not ZIP filenames.

`ip2location` is the same map as `ip2location_db8`. `ip2location_lite` is the same map as `ip2location_db1` / `ip2location_lite_db1`.

Free LITE DB1 (no token) is the reserved `default_ip2location` row.

### Commercial IPv6 BIN

| Package | `file=` | `fieldsPreconfigured` | Record keys |
| --- | --- | --- | --- |
| DB1 | `DB1BINIPV6` | `ip2location_db1` | country, country_name |
| DB2 | `DB2BINIPV6` | `ip2location_db2` | country, country_name, isp |
| DB3 | `DB3BINIPV6` | `ip2location_db3` | country, country_name, region, city |
| DB4 | `DB4BINIPV6` | `ip2location_db4` | country, country_name, region, city, isp |
| DB5 | `DB5BINIPV6` | `ip2location_db5` | country, country_name, region, city |
| DB6 | `DB6BINIPV6` | `ip2location_db6` | country, country_name, region, city, isp |
| DB7 | `DB7BINIPV6` | `ip2location_db7` | country, country_name, region, city, isp, domain |
| DB8 | `DB8BINIPV6` | `ip2location_db8` | country, country_name, region, city, isp, domain |
| DB9 | `DB9BINIPV6` | `ip2location_db9` | country, country_name, region, city |
| DB10 | `DB10BINIPV6` | `ip2location_db10` | country, country_name, region, city, isp, domain |
| DB11 | `DB11BINIPV6` | `ip2location_db11` | country, country_name, region, city |
| DB12 | `DB12BINIPV6` | `ip2location_db12` | country, country_name, region, city, isp, domain |
| DB13 | `DB13BINIPV6` | `ip2location_db13` | country, country_name, region, city |
| DB14 | `DB14BINIPV6` | `ip2location_db14` | country, country_name, region, city, isp, domain |
| DB15 | `DB15BINIPV6` | `ip2location_db15` | country, country_name, region, city |
| DB16 | `DB16BINIPV6` | `ip2location_db16` | country, country_name, region, city, isp, domain |
| DB17 | `DB17BINIPV6` | `ip2location_db17` | country, country_name, region, city |
| DB18 | `DB18BINIPV6` | `ip2location_db18` | country, country_name, region, city, isp, domain |
| DB19 | `DB19BINIPV6` | `ip2location_db19` | country, country_name, region, city, isp, domain |
| DB20 | `DB20BINIPV6` | `ip2location_db20` | country, country_name, region, city, isp, domain |
| DB21 | `DB21BINIPV6` | `ip2location_db21` | country, country_name, region, city |
| DB22 | `DB22BINIPV6` | `ip2location_db22` | country, country_name, region, city, isp, domain |
| DB23 | `DB23BINIPV6` | `ip2location_db23` | country, country_name, region, city, isp, domain |
| DB24 | `DB24BINIPV6` | `ip2location_db24` | country, country_name, region, city, isp, domain |
| DB25 | `DB25BINIPV6` | `ip2location_db25` | country, country_name, region, city, isp, domain |
| DB26 | `DB26BINIPV6` | `ip2location_db26` | country, country_name, region, city, isp, domain, asn |

### LITE IPv6 BIN

Same token URL with `DBnLITEBINIPV6`.

| Package | `file=` | `fieldsPreconfigured` | Record keys |
| --- | --- | --- | --- |
| LITE DB1 | `DB1LITEBINIPV6` | `ip2location_lite_db1` | country, country_name |
| LITE DB2 | `DB2LITEBINIPV6` | `ip2location_lite_db2` | country, country_name, isp |
| LITE DB3 | `DB3LITEBINIPV6` | `ip2location_lite_db3` | country, country_name, region, city |
| LITE DB4 | `DB4LITEBINIPV6` | `ip2location_lite_db4` | country, country_name, region, city, isp |
| LITE DB5 | `DB5LITEBINIPV6` | `ip2location_lite_db5` | country, country_name, region, city |
| LITE DB6 | `DB6LITEBINIPV6` | `ip2location_lite_db6` | country, country_name, region, city, isp |
| LITE DB7 | `DB7LITEBINIPV6` | `ip2location_lite_db7` | country, country_name, region, city, isp, domain |
| LITE DB8 | `DB8LITEBINIPV6` | `ip2location_lite_db8` | country, country_name, region, city, isp, domain |
| LITE DB9 | `DB9LITEBINIPV6` | `ip2location_lite_db9` | country, country_name, region, city |
| LITE DB10 | `DB10LITEBINIPV6` | `ip2location_lite_db10` | country, country_name, region, city, isp, domain |
| LITE DB11 | `DB11LITEBINIPV6` | `ip2location_lite_db11` | country, country_name, region, city |
| ASN LITE | `DBASNLITEBINIPV6` | `ip2location_asn` | asn |

ASN LITE ships no `defaultFile`. Set `path` or let auto-update write a dated file.

| Shortcut | Same map as |
| --- | --- |
| `ip2location` | `ip2location_db8` |
| `ip2location_lite` | `ip2location_db1` / `ip2location_lite_db1` |

## IPinfo (`mmdb`)

Token download (follow redirects):

`https://ipinfo.io/data/<file>.mmdb?token=YOUR_TOKEN`

`archive: none` on those permalinks. Official field lists: [IPinfo database types](https://ipinfo.io/developers/database-types), [Core](https://ipinfo.io/developers/ipinfo-core-database), [Plus](https://ipinfo.io/developers/ipinfo-plus-database).

| Product | Download file | `fieldsPreconfigured` | Record keys | Notes |
| --- | --- | --- | --- | --- |
| Lite | `ipinfo_lite.mmdb` | `ipinfo_lite` | country (`country_code`), country_name (`country`), continent, continent_code, isp (`as_name`), domain (`as_domain`), asn | No region/city. Reserved `default_ipinfo` uses this map. |
| Core | `ipinfo_core.mmdb` | `ipinfo_core` | Lite keys plus region, city | Extra Core columns (lat/long, postal, flags) unused. |
| Plus | `ipinfo_plus.mmdb` | `ipinfo_plus` | Same Record keys as Core | Plus privacy/carrier/change columns unused. |

ASN values keep the official `AS15169` form (string). Do not set `type: uint32` on IPinfo `asn`.

## MaxMind (`mmdb`)

Official GeoLite2 / GeoIP2 permalink (HTTP Basic: account ID + license key):

`https://download.maxmind.com/geoip/databases/{EDITION_ID}/download?suffix=tar.gz`

`archive: tar.gz`. Edition IDs include `GeoLite2-Country`, `GeoLite2-City`, `GeoLite2-ASN`, and paid `GeoIP2-Country`, `GeoIP2-City`, `GeoIP2-ASN`, `GeoIP2-ISP`, `GeoIP2-Domain`, `GeoIP2-Enterprise`.

Official binary fields: [Country](https://dev.maxmind.com/geoip/docs/databases/city-and-country/country-binary/), [ISP](https://dev.maxmind.com/geoip/docs/databases/isp/binary/), [Domain](https://dev.maxmind.com/geoip/docs/databases/domain/binary/), [Enterprise](https://dev.maxmind.com/geoip/docs/databases/enterprise/binary/).

| Product | Typical edition | `fieldsPreconfigured` | Aliases | Record keys |
| --- | --- | --- | --- | --- |
| Country | `GeoLite2-Country` / `GeoIP2-Country` | `maxmind_country` | `geolite2_country`, `geoip2_country` | country (`country.iso_code`), country_name (`country.names.en`), continent (`continent.names.en`), continent_code (`continent.code`) |
| City | `GeoLite2-City` / `GeoIP2-City` | `maxmind_city` | `geolite2_city`, `geoip2_city` | Country keys plus region (`subdivisions.0.iso_code`), city (`city.names.en`) |
| ASN | `GeoLite2-ASN` / `GeoIP2-ASN` | `maxmind_asn` | `geolite2_asn`, `geoip2_asn` | asn (`autonomous_system_number`, uint32, written with `AS` prefix), isp (`autonomous_system_organization`) |
| ISP | `GeoIP2-ISP` (paid) | `maxmind_isp` | `geoip2_isp` | isp (`isp`), asn (`autonomous_system_number`, uint32) |
| Domain | `GeoIP2-Domain` (paid) | `maxmind_domain` | `geoip2_domain` | domain (`domain`) |
| Enterprise | `GeoIP2-Enterprise` (paid) | `maxmind_enterprise` | `geoip2_enterprise` | City keys plus isp (`traits.isp`), domain (`traits.domain`), asn (`traits.autonomous_system_number`, uint32) |

Reserved `default_maxmind` and `default_geolite` use `maxmind_country`. There is no GeoLite2 ISP, Domain, or Enterprise edition.

Match the preset to the file. A Country MMDB with `maxmind_city` leaves region/city empty. An ISP MMDB with `maxmind_country` leaves country empty.
