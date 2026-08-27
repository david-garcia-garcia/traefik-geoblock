# IPinfo Lite database

Facts about IPinfo Lite as a downloadable geo database: license, token download, formats, schema vs this plugin’s `Record`, and how official docs say to read it in Go. Not how this plugin should wrap it.

Local `Record` fields used only for the mapping section: `Country`, `Region`, `City`, `Isp`, `Domain`, `Asn` (`pkg/dbprovider/provider.go`).

## Download: token required, not anonymous

The full IPinfo Lite files are **not** publicly downloadable without an account token. Official curl examples all take `?token=$TOKEN` (or `$YOUR_TOKEN`). Signup is a free IPinfo account; the signup page says Lite API and Database access, no credit card. The token lives on the account dashboard.

Official URL pattern for the current Lite product ([IPinfo Lite Database](https://ipinfo.io/developers/ipinfo-lite-database)):

```
https://ipinfo.io/data/ipinfo_lite.{ext}?token=$TOKEN
```

| Format | Filename | Official curl |
| --- | --- | --- |
| CSV (gzip) | `ipinfo_lite.csv.gz` | `curl -L https://ipinfo.io/data/ipinfo_lite.csv.gz?token=$TOKEN -o ipinfo_lite.csv.gz` |
| MMDB | `ipinfo_lite.mmdb` | `curl -L https://ipinfo.io/data/ipinfo_lite.mmdb?token=$TOKEN -o ipinfo_lite.mmdb` |
| JSON (gzip / NDJSON) | `ipinfo_lite.json.gz` | `curl -L https://ipinfo.io/data/ipinfo_lite.json.gz?token=$TOKEN -o ipinfo_lite.json.gz` |
| Parquet | `ipinfo_lite.parquet` | `curl -L https://ipinfo.io/data/ipinfo_lite.parquet?token=$TOKEN -o ipinfo_lite.parquet` |

`curl` must follow redirects (`-L`). [IP Database Download](https://ipinfo.io/developers/database-download) owns the delivery: `https://ipinfo.io/data/...` returns HTTP 302 to a short-lived signed CDN URL (`dl.assets.ipinfo.io` or `dl.ipinfo.io`).

**Daily updates:** [IPinfo Lite](https://ipinfo.io/lite) says the data is updated daily / every 24 hours. The Lite database page showed file metadata “Last Updated: Aug 26, 2026”. The sample-repo Lite README also says the dataset is updated daily. A vendor community announcement says the database is updated at most once a day (weaker rank; does not contradict).

**Formats:** official Lite product ships CSV, MMDB, JSON (NDJSON), and Parquet. Same four on [IP Database Download](https://ipinfo.io/developers/database-download). JSON is newline-delimited JSON, not one giant array.

Older free downloads (`country`, `country_asn`, `asn`) still exist under `https://ipinfo.io/data/free/{name}.{ext}?token=$TOKEN`. The Lite database page calls `country_asn` the previous schema (start_ip/end_ip). The official CLI `ipinfo download` documents `asn`, `country`, `country-asn` — not the `ipinfo_lite.*` slugs. Those are different filename families, not a disagreement about Lite’s own URLs.

Extracts: [.sources/ipinfo-lite-database.md](.sources/ipinfo-lite-database.md), [.sources/database-download.md](.sources/database-download.md), [.sources/ipinfo-lite.md](.sources/ipinfo-lite.md), [.sources/signup.md](.sources/signup.md), [.sources/developers.md](.sources/developers.md), [.sources/cli.md](.sources/cli.md)

## License (CC-BY-SA 4.0) and bundling a sample

IPinfo states Lite data downloads are released under **Creative Commons Attribution-ShareAlike 4.0 International**. Official attribution they ask for is a credit/link to https://ipinfo.io (example HTML on the Lite database page). The Lite marketing FAQ says you may use Lite data in a product, including commercial use, if you attribute IPinfo.

CC-BY-SA 4.0 deed ([creativecommons.org/licenses/by-sa/4.0/](https://creativecommons.org/licenses/by-sa/4.0/)) grants Share (copy and redistribute, any medium, **even commercially**) and Adapt (even commercially), if you give attribution, mark changes, and ShareAlike adapted material. The legal code Section 2(a)(1) grants reproduce and Share in whole or in part, and produce/Share Adapted Material. Section 4 covers sui generis database rights (extract/reuse/share a substantial portion).

**Can a GitHub repo bundle a sample or lite file?** IPinfo already publishes 100-row Lite samples on GitHub (`ipinfo/sample-database`, folder `IPinfo Lite/`) and says sample datasets may be downloaded **without** an access token ([filename reference](https://ipinfo.io/developers/database-filename-reference)). That is IPinfo redistributing samples themselves. CC-BY-SA 4.0 Share also covers redistributing licensed material with attribution. This folder is not legal advice; it only records what those owners say.

Issue #29 (ticket) claims the license permits commercial use, redistribution, and repackaging. Official IPinfo + CC-BY-SA already cover commercial use and Share; “repackaging” is not a word IPinfo’s license page uses. Follow official + CC. Ticket must not be the sole owner.

Extracts: [.sources/ipinfo-lite-database.md](.sources/ipinfo-lite-database.md), [.sources/ipinfo-lite.md](.sources/ipinfo-lite.md), [.sources/cc-by-sa-4.0.md](.sources/cc-by-sa-4.0.md), [.sources/cc-by-sa-4.0-legalcode.md](.sources/cc-by-sa-4.0-legalcode.md), [.sources/database-filename-reference.md](.sources/database-filename-reference.md)

## Schema vs plugin `Record`

Official Lite fields ([IPinfo Lite Database](https://ipinfo.io/developers/ipinfo-lite-database); same table on `ipinfo/sample-database` Lite README):

| Lite field | Example | Meaning |
| --- | --- | --- |
| `network` | `154.24.39.204/30` | CIDR / range / single IP |
| `country` | `Canada` | Country **name** |
| `country_code` | `CA` | ISO 3166-1 alpha-2 |
| `continent` | `North America` | Continent name |
| `continent_code` | `NA` | Two-letter continent code |
| `asn` | `AS174` | ASN, including the `AS` prefix in examples |
| `as_name` | `Cogent Communications` | ASN organization name |
| `as_domain` | `cogentco.com` | ASN organization domain |

Lite is country + continent + ASN. It does **not** have `region`, `city`, or an `isp` column. Those appear on IPinfo Core / Plus ([IP Database Types](https://ipinfo.io/developers/database-types); Core sample README lists `city` and `region`).

Mapping to this plugin’s `Record` (authority: **inference** from official schema + `pkg/dbprovider/provider.go`; IPinfo does not mention this plugin):

| `Record` | Lite | Notes |
| --- | --- | --- |
| `Country` | `country_code` | Plugin allow/block already uses ISO alpha-2 (`US`, `AU`). Lite `country` is the full name — do not put the name in `Country` if callers expect ISO. |
| `Region` | *(none)* | Empty, same as IP2Location LITE DB1. |
| `City` | *(none)* | Empty. |
| `Isp` | no `isp` field | Closest Lite column is `as_name` (ASN org). That is not documented as ISP. |
| `Domain` | `as_domain` | ASN org website, not a hosted-domains product. |
| `Asn` | `asn` | Official examples are `AS174` / `AS15169`. This plugin’s IP2Location ASN path stores a numeric string (`15169`). Format is not the same. |
| *(no field)* | `continent`, `continent_code`, `country` (name) | Lite-only extras. |

Alternate `country_asn` schema uses `country` as the ISO code and `country_name` for the name — do not mix schemas.

[IP Database Types](https://ipinfo.io/developers/database-types) swaps the prose labels for `as_name` / `as_domain` (“organization domain name” vs “name of the ASN organization”) relative to the Lite product page. Keep both extracts. Follow the Lite database page for Lite (product-specific official).

Sample CSV/JSON rows match the Lite field names (`country_code=AU` for `1.0.0.0/24`). Some rows have empty/null ASN columns.

Extracts: [.sources/ipinfo-lite-database.md](.sources/ipinfo-lite-database.md), [.sources/database-types.md](.sources/database-types.md), [.sources/IPinfo-Lite-README.md](.sources/IPinfo-Lite-README.md), [.sources/ipinfo_lite_sample.csv.md](.sources/ipinfo_lite_sample.csv.md), [.sources/sample-database-README.md](.sources/sample-database-README.md)

## Official Go libraries

### `github.com/ipinfo/go` — API client, not a local DB reader

Official IPinfo developer hub lists official SDKs including Go. The official Go README (`ipinfo/go@7483de07572f709126bc2ff0158afaf3c15af9b0:README.md`) calls the module the **IPinfo Go Client Library** for the **IP address API**. Install: `go get github.com/ipinfo/go/v2/ipinfo`. `go.mod` is `github.com/ipinfo/go/v2`, Go 1.18; no MMDB dependency.

Lite API support: `NewLiteClient` / `GetIPInfoLite` against `https://api.ipinfo.io/lite/` (`ipinfo/lite.go`). Token still required. Returned JSON fields match the Lite API (`country_code`, `asn`, `as_name`, `as_domain`, …). There is **no** local MMDB/CSV open path in that repo (search of `*.go` found no `mmdb` / MaxMind usage).

The same README’s “free plan is limited to 50,000 requests per month” is the **Core / classic** API free tier, not Lite. Lite API quota is owned by the Lite API docs (unlimited). Conflict: keep both; follow Lite API docs for `/lite`.

### How IPinfo documents reading MMDB / CSV locally in Go

Official [IP Database Download](https://ipinfo.io/developers/database-download): use an MMDB reader library, open the file, look up an IP; IPinfo’s `mmdbctl` can `read` an IP from an `.mmdb` (example uses `location.mmdb`, not `ipinfo_lite.mmdb`). Official CLI: `ipinfo mmdb read`. Official IPinfo Community article (vendor) for Go: `go get github.com/oschwald/maxminddb-golang`, `maxminddb.Open`, `db.Lookup(ip, &result)` with struct tags taken from the IPinfo schema. That article’s sample struct is the **privacy** database (`hosting`, `proxy`, …), not Lite `country_code`.

No official IPinfo page publishes a Go snippet that looks up Lite `country_code` from MMDB. Closest official pieces: schema field name `country_code`; `mmdbctl read`; vendor Go article saying tags must match the documented schema.

A full `ipinfo_lite.mmdb` opened with oschwald v2 (`Lookup().Decode` into `any`) returns a **flat** map. Tags such as `` `maxminddb:"country_code"` `` match this file. oschwald’s README `DecodePath(..., "country", "iso_code")` is a GeoIP2 example, not IPinfo Lite. See § On-disk Lite MMDB.

CSV: official docs treat CSV as rows for pipelines / grep, not a Go lookup API. No official IPinfo-owned Go CSV geo reader.

### `github.com/oschwald/maxminddb-golang` vs an IPinfo-owned MMDB reader

IPinfo does **not** publish an official Go MMDB reader. Official download docs say “MMDB reader libraries supported by IPinfo” (the library list did not appear in the fetched HTML). IPinfo Community (vendor) names `github.com/oschwald/maxminddb-golang`. That library’s README (source) says it works with IPinfo `.mmdb` files.

Current default branch of that repo is **v2** (`github.com/oschwald/maxminddb-golang/v2`, `go 1.25.0`). API: `db.Lookup(ip).Decode(&record)` with `netip.Addr`. The IPinfo Community article still shows the **v1** API (`db.Lookup(ip, &result)`, `net.IP`). Follow **source** for what v2 does; the vendor article describes v1.

### Official sample for MMDB `country_code`

None in official IPinfo Go docs. Official CLI/mmdbctl examples look up an IP and print JSON. Sample Lite MMDB file exists in `ipinfo/sample-database` (`IPinfo Lite/ipinfo_lite_sample.mmdb`).

Extracts: [.sources/ipinfo-go-README.md](.sources/ipinfo-go-README.md), [.sources/ipinfo-go-lite.go.md](.sources/ipinfo-go-lite.go.md), [.sources/ipinfo-go-go.mod.md](.sources/ipinfo-go-go.mod.md), [.sources/database-download.md](.sources/database-download.md), [.sources/using-ipinfos-mmdb-data-downloads-with-golang.md](.sources/using-ipinfos-mmdb-data-downloads-with-golang.md), [.sources/maxminddb-golang-README.md](.sources/maxminddb-golang-README.md)

## Sample databases

[IPinfo Lite Database](https://ipinfo.io/developers/ipinfo-lite-database) links the sample-database GitHub repo and lists Lite samples (CSV, JSON, MMDB). [Filename reference](https://ipinfo.io/developers/database-filename-reference): each sample is 100 rows, updated daily, same data across formats; **no access token** required for samples.

Clone `ipinfo/sample-database@ff663e000ab0fe32e28b5911be262a01cf284d9a` folder `IPinfo Lite/`:

- `ipinfo_lite_sample.csv`
- `ipinfo_lite_sample.json`
- `ipinfo_lite_sample.mmdb`
- `ipinfo_lite_sample.parquet`
- `README.md`

Extracts: [.sources/sample-database-README.md](.sources/sample-database-README.md), [.sources/IPinfo-Lite-README.md](.sources/IPinfo-Lite-README.md), [.sources/database-filename-reference.md](.sources/database-filename-reference.md)

## Free Lite API vs local database

| | Lite API | Local Lite database |
| --- | --- | --- |
| Endpoint / file | `https://api.ipinfo.io/lite/{ip}?token=$TOKEN` (also `/lite/me`) | `https://ipinfo.io/data/ipinfo_lite.*` |
| Token | Required (query, Basic, or Bearer) | Required for the full file |
| Request quota | Official: no daily/monthly limit; unlimited | Local lookups are unlimited once the file is on disk (IPinfo Lite page / blog). **Download** cap: **10 downloads per day per unique IP** ([IP Database Download](https://ipinfo.io/developers/database-download)). Checksums do not count toward that cap. |
| Schema | Same country/ASN fields as the DB, plus `ip` | Table above; no `ip` column (key is `network`) |

Do not use the Core API 50k/month figure for Lite.

Extracts: [.sources/lite-api.md](.sources/lite-api.md), [.sources/developers.md](.sources/developers.md), [.sources/database-download.md](.sources/database-download.md), [.sources/ipinfo-lite.md](.sources/ipinfo-lite.md)

## MMDB mmap / unsafe (Yaegi-relevant later)

Pinned: `github.com/oschwald/maxminddb-golang@a295f5b07f9e2e131ede7db07dbd592a225028c0` (module v2).

`Open` **memory-maps** the file on supported platforms (`reader.go`). If mmap is unsupported (comment: WebAssembly, App Engine, or filesystem), it falls back to reading the file into memory and `OpenBytes`. `OpenBytes` takes a `[]byte` and does not mmap.

Unix mmap (`mmap_unix.go`): `golang.org/x/sys/unix.Mmap` / `Munmap`. No `unsafe` import on that file.

Windows mmap (`mmap_windows.go`): imports `unsafe`; uses `unsafe.Slice` / `unsafe.Pointer` to turn the mapped address into `[]byte`.

Decoder comment (`data_decoder.go`): map-key decoding **previously** used `unsafe`; current code says it no longer does.

`go.mod`: `go 1.25.0`; requires `golang.org/x/sys`.

`github.com/ipinfo/go` has no mmap/unsafe MMDB path.

**Yaegi (measured 2026-08-27, Traefik v3.7.11 / yaegi v0.16.1):** importing upstream v1.13.1 `Open` pulls `golang.org/x/sys/unix`. Plugin load panics: `ifreq_linux.go:25:24: incomplete type ifreq`. `FromBytes` / `os.ReadFile` does not mmap. After `go mod vendor`, delete `mmap_*.go` and overlay `reader_mmap.go` (`scripts/apply-oschwald-yaegi-patch.ps1`) so Yaegi never parses `x/sys`. Do not fork the decoder.

Extracts: [.sources/reader.go.md](.sources/reader.go.md), [.sources/mmap_unix.go.md](.sources/mmap_unix.go.md), [.sources/mmap_windows.go.md](.sources/mmap_windows.go.md), [.sources/maxminddb-golang-go.mod.md](.sources/maxminddb-golang-go.mod.md), [.sources/data_decoder.go.md](.sources/data_decoder.go.md)

## On-disk Lite MMDB

Operator-downloaded official `ipinfo_lite.mmdb` (23,496,022 bytes), opened with `github.com/oschwald/maxminddb-golang/v2` v2.5.0. Metadata: `DatabaseType=ipinfo bundle_location_lite.mmdb`, `IPVersion=6`, `NodeCount=2015374`, `RecordSize=32`, `BuildEpoch=1787731420` (2026-08-26 08:03 UTC).

Decoded keys for `8.8.8.8`: `country`, `country_code`, `continent`, `continent_code`, `asn`, `as_name`, `as_domain`. Values: `United States` / `US` / `North America` / `NA` / `AS15169` / `Google LLC` / `google.com`. Prefix on the lookup result: `8.8.8.0/24`. A `network` struct tag decoded empty — that CSV column is not in the MMDB payload.

`127.0.0.1` and `192.168.1.1`: `Found()=false`, decoded record `null`. `1.1.1.1` → `country_code=AU` (same as this plugin’s IP2Location LITE fixture for that IP).

Extract: [.sources/ipinfo_lite.mmdb-probe.md](.sources/ipinfo_lite.mmdb-probe.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Full Lite download URL `ipinfo_lite.*` + `token` | https://ipinfo.io/developers/ipinfo-lite-database | official |
| Formats CSV, MMDB, JSON, Parquet | same + database-download | official |
| 302 to signed CDN; `curl -L` | https://ipinfo.io/developers/database-download | official |
| Daily / 24h refresh | https://ipinfo.io/lite | official |
| Account/signup for Lite API and Database | https://ipinfo.io/signup | official |
| Token on dashboard; Lite API auth methods | https://ipinfo.io/developers | official |
| Lite API unlimited; token still required | https://ipinfo.io/developers/lite-api | official |
| Download cap 10/day/IP | https://ipinfo.io/developers/database-download | official |
| CC-BY-SA 4.0 + IPinfo attribution | Lite database page + https://ipinfo.io/lite | official |
| Share/Adapt commercially, attribution, ShareAlike | https://creativecommons.org/licenses/by-sa/4.0/ | official |
| Samples without token; 100 rows | https://ipinfo.io/developers/database-filename-reference | official |
| Sample files on disk | ipinfo/sample-database@ff663e00 | source |
| Lite field list (no region/city) | Lite database page | official |
| Core/Plus have city/region | https://ipinfo.io/developers/database-types | official |
| `github.com/ipinfo/go` is API-only | ipinfo/go@7483de07 README + lite.go | official + source |
| Go MMDB via oschwald | IPinfo Community article | vendor |
| oschwald Open mmaps; OpenBytes does not; Windows mmap uses unsafe; go 1.25 | oschwald@a295f5b0 | source |
| Community Go sample uses v1 Lookup(ip, &T) | same Community article | vendor |
| Current oschwald API is v2 Lookup().Decode | oschwald README @a295f5b0 | source |
| Map Lite columns → plugin `Record` | inferred from official schema + `pkg/dbprovider/provider.go` | inference |
| Lite MMDB records are flat (`country_code`, not GeoIP2 nest); no `network` in payload; private IPs not found | operator `ipinfo_lite.mmdb` + oschwald v2.5.0 decode | source |
| License allows “repackaging” | issue #29 only | ticket (do not follow as sole owner) |

Conflicts:

1. **Lite API quota vs go README 50k/month** — Lite API official page (unlimited) wins for `/lite`. The 50k figure is the classic/Core free plan on the Go README.
2. **`as_name` / `as_domain` labels** on database-types vs Lite product page — follow Lite product page.
3. **oschwald v1 (vendor article) vs v2 (current source)** — follow source for this commit; vendor article is stale on API shape.
4. **CLI `ipinfo download` names vs `ipinfo_lite.*`** — different filename families; Lite product page owns Lite slugs.

## References

- https://ipinfo.io/developers/ipinfo-lite-database
- https://ipinfo.io/lite
- https://ipinfo.io/developers/database-download
- https://ipinfo.io/developers/lite-api
- https://ipinfo.io/developers
- https://ipinfo.io/developers/database-types
- https://ipinfo.io/developers/database-filename-reference
- https://ipinfo.io/developers/cli
- https://ipinfo.io/signup
- https://creativecommons.org/licenses/by-sa/4.0/
- https://creativecommons.org/licenses/by-sa/4.0/legalcode.en
- https://github.com/ipinfo/sample-database/blob/ff663e000ab0fe32e28b5911be262a01cf284d9a/IPinfo%20Lite/README.md
- https://github.com/ipinfo/go/blob/7483de07572f709126bc2ff0158afaf3c15af9b0/README.md
- https://github.com/oschwald/maxminddb-golang/blob/a295f5b07f9e2e131ede7db07dbd592a225028c0/reader.go
- https://community.ipinfo.io/t/using-ipinfos-mmdb-data-downloads-with-golang/4415
- https://github.com/david-garcia-garcia/traefik-geoblock/issues/29
