# GeoLite2 database

Facts about official MaxMind GeoLite2 / GeoIP2 downloadable MMDB files: account download, edition IDs, Country/City/ASN schema (nested `country.iso_code`), license limits on redistributing a live database, and how MaxMind lists Go readers versus `oschwald/maxminddb-golang`. Not how this plugin should wrap a provider.

Local `Record` fields used only for the mapping section: `Country`, `Region`, `City`, `Isp`, `Domain`, `Asn` (`pkg/dbprovider/provider.go`). This plugin already opens MMDB with vendored `github.com/oschwald/maxminddb-golang` v1.13.1 (`FromBytes`).

## Download: account ID + license key, not anonymous

GeoLite files are **not** anonymous public downloads. Official GeoLite docs: sign up for a MaxMind account, generate a license key, then download or call `geoipupdate`. Signup: [GeoLite2 signup](https://www.maxmind.com/en/geolite2/signup). Keys: [Generate a license key](https://support.maxmind.com/knowledge-base/articles/generate-a-maxmind-license-key). Treat the key as a password ([Using MaxMind license keys](https://support.maxmind.com/knowledge-base/articles/using-maxmind-license-keys)). You also need the numeric [account ID](https://support.maxmind.com/knowledge-base/articles/find-your-maxmind-account-id).

MaxMind recommends [GeoIP Update](https://dev.maxmind.com/geoip/updating-databases/) for binary MMDB. Direct download is for systems that cannot run it, and for CSV.

**Permalink / direct URL shape** ([Updating GeoIP and GeoLite Databases](https://dev.maxmind.com/geoip/updating-databases/)):

```
https://download.maxmind.com/geoip/databases/{EDITION_ID}/download?suffix={suffix}
```

- Binary MMDB: `suffix=tar.gz` (gzip). CSV: `suffix=zip`.
- Authenticate with **HTTP Basic Auth**: username = account ID, password = license key.
- Official `curl` follows redirects (`-L`) and uses `-u YOUR_ACCOUNT_ID:YOUR_LICENSE_KEY`. Official example edition is `GeoIP2-City-CSV` with `suffix=zip`.
- Clients are redirected to an R2 presigned URL on `mm-prod-geoip-databases.a2649acb697e2c09b632799562c076f2.r2.cloudflarestorage.com`. Do not block that host.
- Hosts listed for direct API downloads: `https://download.maxmind.com` and `https://updates.maxmind.com` ([Download and update databases](https://support.maxmind.com/knowledge-base/articles/download-and-update-maxmind-databases)).
- Account-portal permalinks live on [Download Databases](https://www.maxmind.com/en/accounts/current/geoip/downloads) (login). HEAD on a permalink does not count against the daily download cap.

**GeoLite edition IDs** (binary), from official `geoipupdate` Docker compose examples (`maxmind/geoipupdate` `doc/docker.md`):

| Edition ID | Product |
| --- | --- |
| `GeoLite2-Country` | GeoLite Country MMDB |
| `GeoLite2-City` | GeoLite City MMDB |
| `GeoLite2-ASN` | GeoLite ASN MMDB |

`geoipupdate` `EditionIDs` is a space-separated list of those IDs (`GeoIP.conf`). CSV zip names on the City/Country and ASN docs are `{GeoIP2,GeoLite2}-{City,Country}-CSV_{YYYYMMDD}.zip` and `GeoLite2-ASN-CSV_{YYYYMMDD}.zip`, so CSV permalink editions follow the same `{name}-CSV` pattern as the official `GeoIP2-City-CSV` example.

**Limits:** GeoLite users are limited to **30 database downloads per day** ([GeoLite Databases and Web Services](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)). The download KB says every account is limited to 1,000 total direct downloads (**30 for GeoLite accounts**) in 24 hours. HEAD / “check for update” requests do not count. GeoLite Country and City update **Tuesday and Friday**; GeoLite ASN updates **weekdays**.

Extracts: [.sources/updating-databases.md](.sources/updating-databases.md), [.sources/geolite2-free-geolocation-data.md](.sources/geolite2-free-geolocation-data.md), [.sources/download-and-update-maxmind-databases.md](.sources/download-and-update-maxmind-databases.md), [.sources/using-maxmind-license-keys.md](.sources/using-maxmind-license-keys.md), [.sources/generate-a-maxmind-license-key.md](.sources/generate-a-maxmind-license-key.md), [.sources/geoipupdate-GeoIP.conf.md](.sources/geoipupdate-GeoIP.conf.md), [.sources/geoipupdate-docker.md](.sources/geoipupdate-docker.md)

## Free databases: Country, City, ASN

Official GeoLite ships **three** downloadable databases ([GeoLite Databases and Web Services](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)):

| Database | Official use | Has ISO country? |
| --- | --- | --- |
| GeoLite Country | Country-level geolocation for analytics, content customization, or compliance in territories that are not disputed | Yes — binary `country.iso_code` |
| GeoLite City | City / postal; official note: considerably less accurate than paid GeoIP City; not recommended for commercial use cases | Yes — same nested country map (City includes Country fields) |
| GeoLite ASN | Autonomous system number and organization | **No** country fields |

Paid GeoIP Country / City use the same binary field layout. Some fields documented on the shared City/Country page are absent or empty in GeoLite (example: CSV `is_anycast` is empty in GeoLite2-Country and GeoLite2-City). `country.iso_code` / CSV `country_iso_code` has no GeoLite exclusion.

Formats: binary `.mmdb` (fast lookups) and CSV. GeoLite EULA requires deleting a GeoLite database **within 30 days of a new release**.

Extracts: [.sources/geolite2-free-geolocation-data.md](.sources/geolite2-free-geolocation-data.md), [.sources/city-and-country.md](.sources/city-and-country.md), [.sources/asn.md](.sources/asn.md), [.sources/asn-binary.md](.sources/asn-binary.md)

## Country field path: `country.iso_code` (not IPinfo `country_code`)

GeoIP2 / GeoLite2 **binary** Country records are a **nested map**. Top-level keys include `continent`, `country`, `registered_country`, `represented_country`, `traits`. Under `country`:

| Key | Type | Meaning |
| --- | --- | --- |
| `iso_code` | string | Two-character [ISO 3166-1](https://en.wikipedia.org/wiki/ISO_3166-1) alpha code |
| `geoname_id` | uint32 | GeoNames id |
| `names` | map | Locale → localized name |
| `is_in_european_union` | boolean | Present only when true |

Owner: [GeoIP Country binary database fields](https://dev.maxmind.com/geoip/docs/databases/city-and-country/country-binary/).

oschwald `geoip2-golang` v1 structs match that nest: `` Country `maxminddb:"country"` `` then `` IsoCode `maxminddb:"iso_code"` ``. Lookup path is `country` / `iso_code`, not a flat `country_code`.

IPinfo Lite MMDB is a **flat** map with `country_code` (ISO) and `country` (full name). Those tags do **not** match GeoLite2. A struct written for IPinfo Lite will not populate ISO from a GeoLite2-Country file.

CSV Locations use `country_iso_code` (string, 2). MaxMind tells you not to key on `*_name` fields; use `country_iso_code` for country.

ASN binary is only `autonomous_system_number` and `autonomous_system_organization`. No ISO country.

Extracts: [.sources/country-binary.md](.sources/country-binary.md), [.sources/city-and-country.md](.sources/city-and-country.md), [.sources/asn-binary.md](.sources/asn-binary.md), [.sources/geoip2-golang-v1.13.0-reader.go.md](.sources/geoip2-golang-v1.13.0-reader.go.md)

## GeoLite2-Country is enough for ISO allow/block

Official Country product is “geolocation at the country-level”. The binary `country.iso_code` field is the ISO 3166-1 alpha-2 code MaxMind recommends as the country key. A GeoLite2-Country (or GeoIP2-Country) MMDB is sufficient to decide allow/block by ISO country.

GeoLite2-City also has `country.iso_code` plus city/subdivision. GeoLite2-ASN does not; ASN-only cannot drive country allow/block.

Mapping to this plugin’s `Record` (authority: **inference** from official schema + `pkg/dbprovider/provider.go`; MaxMind does not mention this plugin):

| `Record` | GeoLite2-Country | GeoLite2-City | GeoLite2-ASN |
| --- | --- | --- | --- |
| `Country` | `country.iso_code` | `country.iso_code` | *(none)* |
| `Region` | *(none)* | first `subdivisions[].iso_code` | *(none)* |
| `City` | *(none)* | `city.names` (locale) | *(none)* |
| `Isp` | *(none)* | *(none)* | closest: `autonomous_system_organization` (not documented as ISP) |
| `Domain` | *(none)* | *(none)* | *(none)* |
| `Asn` | *(none)* | *(none)* | `autonomous_system_number` (uint, not IPinfo’s `AS` prefix string) |

Use `country.iso_code`, not localized `country.names`. `registered_country.iso_code` is where the ISP registered the block and may differ from located `country`.

Extracts: [.sources/geolite2-free-geolocation-data.md](.sources/geolite2-free-geolocation-data.md), [.sources/country-binary.md](.sources/country-binary.md)

## License: do not commit a live GeoLite2 MMDB to a public repo

This folder is not legal advice. It records what official MaxMind documents say.

The [GeoLite End User License Agreement](https://www.maxmind.com/en/geolite/eula) (updated 2026-02-12; “GeoLite” includes GeoLite2):

- Copyrightable elements are under [CC-BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). Attribution example: `This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com`.
- Internal-use license is limited to internal business purposes.
- **EULA controls** if it conflicts with CC / DPA / privacy / website terms.
- §6.1: except as CC explicitly permits, do not disclose the Services to a third party without MaxMind’s prior written consent. If you do disclose as permitted, you must impose the same (or substantially similar) duties, including §6.3 destroy-old-versions, and you are responsible for those third parties.
- §6.3: promptly use updated GeoLite databases; **cease use and destroy old versions within 30 days** of a new release.

Vendor KB:

- [Who is covered](https://support.maxmind.com/knowledge-base/articles/who-is-covered-by-the-geolite-end-user-license-agreement): sharing GeoLite requires holding recipients to the EULA, including deleting old databases.
- [Sell or display](https://support.maxmind.com/knowledge-base/articles/sell-or-display-data-from-geolite-databases-and-web-services): EULA allows internal use and building apps / displaying data **outside** the org **with attribution**. Selling the data or apps that include them **without** attribution needs a Commercial License.
- [Commercial License for GeoLite](https://support.maxmind.com/knowledge-base/articles/commercial-license-for-geolite): **including GeoLite database data in a product or service you provide to users or customers** requires a Commercial Redistribution License (paid; per reseller product; end users must get a restrictive EULA).

**Can this public repo commit a real GeoLite2 `.mmdb`?** Official docs do **not** say yes. Publishing a live GeoLite file on GitHub is redistribution of the Services. Recipients of a git history cannot be held to “destroy within 30 days of a new release.” The Commercial License KB is the document that names distributing copies of GeoLite databases as part of a product. Follow EULA + that KB over any ticket or unofficial mirror.

**Official dummy fixtures:** MaxMind publishes **dummy** (not real GeoIP) test MMDBs on GitHub: `GeoIP2-Country-Test.mmdb`, `GeoIP2-City-Test.mmdb`, `GeoLite2-ASN-Test.mmdb` under [maxmind/MaxMind-DB test-data](https://github.com/maxmind/MaxMind-DB/tree/main/test-data) ([City and Country example files](https://dev.maxmind.com/geoip/docs/databases/city-and-country/#example-files)). Those are MaxMind’s own test files, not a live GeoLite download.

Extracts: [.sources/geolite-eula.md](.sources/geolite-eula.md), [.sources/who-is-covered-geolite-eula.md](.sources/who-is-covered-geolite-eula.md), [.sources/sell-or-display-geolite.md](.sources/sell-or-display-geolite.md), [.sources/commercial-license-for-geolite.md](.sources/commercial-license-for-geolite.md), [.sources/city-and-country.md](.sources/city-and-country.md)

## Official Go reader vs oschwald

MaxMind’s [official client APIs](https://dev.maxmind.com/geoip/docs/databases/#official-client-apis) are .NET, C (`libmaxminddb`), Java, Node.js, PHP, Python, Ruby. **Go is not on that list.**

Under **Unofficial Client APIs** (“Use at your own risk”; MaxMind does not support them), MaxMind lists:

- `oschwald/geoip2-golang` — higher-level GeoIP2/GeoLite2 structs
- `IncSW/geoip2`

`github.com/oschwald/maxminddb-golang` is the low-level MMDB reader. Its v1 README (this repo’s vendor, v1.13.1) says it can read GeoLite2 and GeoIP2 **and** “This is not an official MaxMind API.” It points at `geoip2-golang` for a higher-level API. `FromBytes([]byte)` opens from memory and does not mmap. `geoip2-golang` v1 `Open` memory-maps; it also exposes `FromBytes` that delegates to `maxminddb.FromBytes`.

This plugin already vendors oschwald **v1** (Yaegi / go 1.21). Current upstream default of both repos is **v2** (`netip.Addr`, Go 1.25). Follow **source** for the vendored v1 API (`Lookup(ip, &record)`). Do not take v2 as what this tree imports.

For ISO-only lookups, oschwald’s own geoip2 README says you may get better performance by calling `maxminddb.Lookup` with a tiny struct that only has `country.iso_code` tags — you do not need the geoip2 wrapper.

MaxMind’s official Go-related tool is `mmdbinspect` (beta CLI), not a library.

Extracts: [.sources/databases-client-apis.md](.sources/databases-client-apis.md), [.sources/maxminddb-golang-v1-README.md](.sources/maxminddb-golang-v1-README.md), [.sources/maxminddb-golang-v1-reader.go.md](.sources/maxminddb-golang-v1-reader.go.md), [.sources/geoip2-golang-v1.13.0-reader.go.md](.sources/geoip2-golang-v1.13.0-reader.go.md)

## Unofficial mirrors (ticket only)

Issue [#52](https://github.com/david-garcia-garcia/traefik-geoblock/issues/52) says the reporter already downloads MaxMind with their own container and API key, and edits in [P3TERX/GeoLite.mmdb](https://github.com/P3TERX/GeoLite.mmdb) as a way to skip the API key.

That link is **not** an official MaxMind download. Official GeoLite requires an account and license key. Redistributing live GeoLite files without holding recipients to the EULA (including 30-day destroy) conflicts with official EULA §6.1 / §6.3. This plugin does **not** commit a live GeoLite MMDB.

Human 2026-08-29: reserved catalog `default_geolite` MAY GET the Country file from that repo. Verified GET (follow redirects): `https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb` → `raw.githubusercontent.com` … `200` `application/octet-stream` Content-Length `8619527`. City/ASN files on the same branch are not the default. Official permalink + Basic Auth remains the operator example for licensed GeoLite.

Extract: [.sources/issue-52.md](.sources/issue-52.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Account + license key required; GeoLite signup | https://dev.maxmind.com/geoip/geolite2-free-geolocation-data | official |
| Permalink `https://download.maxmind.com/geoip/databases/{EDITION}/download?suffix=` + Basic Auth account:key | https://dev.maxmind.com/geoip/updating-databases/ | official |
| Follow redirects to named R2 host | same | official |
| Binary edition IDs `GeoLite2-Country` `GeoLite2-City` `GeoLite2-ASN` | maxmind/geoipupdate `doc/docker.md` compose example | official (vendor repo) |
| 30 GeoLite downloads / 24h | GeoLite page + download KB | official / vendor |
| Three free DBs: Country, City, ASN | GeoLite page | official |
| Binary country ISO is nested `country.iso_code` | https://dev.maxmind.com/geoip/docs/databases/city-and-country/country-binary/ | official |
| ASN MMDB has no country fields | https://dev.maxmind.com/geoip/docs/databases/asn/binary/ | official |
| Country DB is country-level geo (enough for ISO) | GeoLite page | official |
| Destroy old GeoLite within 30 days of a new release | https://www.maxmind.com/en/geolite/eula §6.3 | official |
| Share must hold recipients to EULA including destroy-old | https://support.maxmind.com/knowledge-base/articles/who-is-covered-by-the-geolite-end-user-license-agreement | vendor |
| Including GeoLite DB in a product you provide needs Commercial Redistribution License | https://support.maxmind.com/knowledge-base/articles/commercial-license-for-geolite | vendor |
| Dummy official test MMDBs on maxmind/MaxMind-DB | City and Country example-files section | official |
| No official Go client API; oschwald/geoip2-golang is unofficial | https://dev.maxmind.com/geoip/docs/databases/ | official |
| maxminddb-golang is unofficial; `FromBytes` exists | vendor `github.com/oschwald/maxminddb-golang` v1.13.1 | source |
| geoip2-golang v1 tags `country` / `iso_code`; `FromBytes` wraps maxminddb | oschwald/geoip2-golang@v1.13.0:reader.go | source |
| Map GeoLite fields → plugin `Record` | inferred from official schema + `pkg/dbprovider/provider.go` | inference |
| P3TERX mirror as key-free download | issue #52 only | ticket (do not follow as sole owner) |

Conflicts:

1. **Sell-or-display KB vs Commercial License KB** — sell-or-display allows outside-org apps / display **with attribution**; commercial-license says including GeoLite **database data in a product you provide to users** needs a paid redistribution license. Keep both. For **committing a live MMDB** (distributing the file), follow EULA §6.1/§6.3 + commercial-license KB. Not legal advice.
2. **oschwald v1 (this tree) vs v2 (current upstream)** — follow vendored v1.13.1 for what this plugin compiles; v2 is a different module path and Go 1.25.

## References

- https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
- https://dev.maxmind.com/geoip/updating-databases/
- https://dev.maxmind.com/geoip/docs/databases/city-and-country/country-binary/
- https://dev.maxmind.com/geoip/docs/databases/city-and-country/
- https://dev.maxmind.com/geoip/docs/databases/asn
- https://dev.maxmind.com/geoip/docs/databases/asn/binary/
- https://dev.maxmind.com/geoip/docs/databases/
- https://www.maxmind.com/en/geolite/eula
- https://www.maxmind.com/en/geolite2/signup
- https://support.maxmind.com/knowledge-base/articles/download-and-update-maxmind-databases
- https://support.maxmind.com/knowledge-base/articles/using-maxmind-license-keys
- https://support.maxmind.com/knowledge-base/articles/generate-a-maxmind-license-key
- https://support.maxmind.com/knowledge-base/articles/who-is-covered-by-the-geolite-end-user-license-agreement
- https://support.maxmind.com/knowledge-base/articles/sell-or-display-data-from-geolite-databases-and-web-services
- https://support.maxmind.com/knowledge-base/articles/commercial-license-for-geolite
- https://github.com/maxmind/geoipupdate/blob/main/doc/GeoIP.conf.md
- https://github.com/maxmind/geoipupdate/blob/main/doc/docker.md
- https://github.com/oschwald/geoip2-golang/blob/v1.13.0/reader.go
- https://github.com/oschwald/maxminddb-golang (module v1.13.1, vendored)
- https://github.com/maxmind/MaxMind-DB/tree/main/test-data
- https://github.com/david-garcia-garcia/traefik-geoblock/issues/52
