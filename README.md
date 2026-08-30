# 🌍 Traefik Geoblock and Geoenrich

[![Build Status](https://github.com/david-garcia-garcia/traefik-geoblock/actions/workflows/ci.yml/badge.svg)](https://github.com/david-garcia-garcia/traefik-geoblock/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/david-garcia-garcia/traefik-geoblock)](https://goreportcard.com/report/github.com/david-garcia-garcia/traefik-geoblock)
[![Latest GitHub release](https://img.shields.io/github/v/release/david-garcia-garcia/traefik-geoblock?sort=semver)](https://github.com/david-garcia-garcia/traefik-geoblock/releases/latest)
[![License](https://img.shields.io/badge/license-Apache%202.0-brightgreen.svg)](LICENSE)

A Traefik middleware that looks up the client IP in a **local** GeoIP database (IP2Location, IPinfo, or MaxMind) and writes country, city, ASN, and related fields onto the **request**. Traefik access logs and your backends become geo-aware — no per-request GeoIP API. The same lookup can **allow or block** by country or CIDR when you want it to.

> [!TIP]
>
> **Traefik Security**
>
> The basic middlewares you need to secure your Traefik ingress:
>
> 🌍 **Geoblock**: [david-garcia-garcia/traefik-geoblock](https://github.com/david-garcia-garcia/traefik-geoblock) — Geo-enrich requests for logs and backends; optionally allow or block by country
> 🛡️ **CrowdSec**: [maxlerebourg/crowdsec-bouncer-traefik-plugin](https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin) — Real-time threat intelligence and automated blocking
> 🔒 **ModSecurity CRS**: [david-garcia-garcia/traefik-modsecurity](https://github.com/david-garcia-garcia/traefik-modsecurity) — Web Application Firewall with OWASP Core Rule Set
> 🚦 **Ratelimit**: [Traefik Rate Limit](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/ratelimit/) — Control request rates and prevent abuse

> [!WARNING]
>
> **Do not run Traefik middlewares as Yaegi plugins in production.**
>
> Traefik's plugin system interprets plugins at request time. For production traffic, compile the middleware into the Traefik binary instead — see [traefik-with-plugins](https://github.com/david-garcia-garcia/traefik-with-plugins). Discussion: [Traefik #12213](https://github.com/traefik/traefik/issues/12213).

## Geo enrichment

Every request that reaches the plugin gets a local GeoIP lookup. The result is written onto the **request** (`requestHeaderEnrich`), not the HTTP response:

- **Traefik access logs** can `keep` those headers so every line is geo-aware — dashboards, SIEM, compliance.
- **Applications** behind Traefik see the same headers (`X-Geo-Country`, city, ASN, …) and can branch on them without calling a GeoIP service.
- Headers are **not** copied onto the response, so browsers and other clients do not see them.

That is the feature most operators want. Blocking is optional.

### Dashboards from access logs

Keep the enrich headers in Traefik JSON access logs and any log pipeline can chart traffic by country, city, ASN, ISP, and allow/block reason. Here is that data in New Relic:

![Request volume, geoblock decisions, traffic by country, and passed vs blocked over time](assets/dashboard-1.jpg)

![Requests by ISP and ASN](assets/dashboard-2.jpg)

![Requests by region and city](assets/dashboard-3.jpg)

![Blocked requests by country and ISP](assets/dashboard-4.jpg)

```yaml
accessLog:
  filePath: "/var/log/traefik/access.log"
  format: json
  fields:
    headers:
      names:
        X-Geo-Country: keep
        X-Geo-Country-Name: keep
        X-Geo-City: keep
        X-Geo-Region: keep
        X-Geo-Asn: keep
        X-Geo-Isp: keep
        X-Geoblock-Decision: keep
```

The backend already received the same request headers. Clients do not.

### Decision reasons (`logStatusDetailHeader`)

Format `{action}:{reason}`. These are the values on `X-Geoblock-Decision` (or whatever name you set) and in the dashboard donut.

**Bypass (blocking skipped, checked first):**

| Value | Description | Example |
| --- | --- | --- |
| `pass:ignore_verb` | HTTP method is in `ignoreVerbs` | OPTIONS CORS preflight |
| `pass:not_included_regex` | `includedPathsRegex` is set and the request did not match | `/docs` when include is `/secure/.*` |
| `pass:excluded_regex` | Request matched `excludedPathsRegex` | `/health` |
| `pass:bypass_header` | Request matched `bypassHeaders` | Internal service with a secret header |

**Allowed by geo rules:**

| Value | Description | Example |
| --- | --- | --- |
| `pass:allow_private` | Private/internal IP and `allowPrivate` is true | `192.168.1.100` |
| `pass:allowed_ip_block` | CIDR in `allowedIPBlocks` or `allowedIPBlocksDir` | Trusted partner `203.0.113.50` |
| `pass:allowed_country` | Country is in `allowedCountries` | US when `allowedCountries: ["US"]` |
| `pass:default_allow` | No rule matched and `defaultAllow` is true | Unknown country, permissive config |
| `pass:none` | No IP found to evaluate | Misconfigured `ipHeaders` |

**Denied:**

| Value | Description | Example |
| --- | --- | --- |
| `block:allow_private` | Private/internal IP and `allowPrivate` is false | Internal IP, strict config |
| `block:blocked_ip_block` | CIDR in `blockedIPBlocks` or `blockedIPBlocksDir` | Known bad range |
| `block:blocked_country` | Country is in `blockedCountries` | Request from a blocked region |
| `block:default_allow` | No rule matched and `defaultAllow` is false | Unknown country, strict config |
| `block:error` | Lookup failed (or `countryHeader` missing in `block` mode) and `banIfError` is true | Database lookup failure |

### Enrich configuration

```yaml
http:
  middlewares:
    geo-enrich:
      plugin:
        geoblock:
          mode: enrich
          countryHeader: X-IPCountry
          requestHeaderEnrich:
            X-IPCountry: country
            X-Geo-Country: country
            X-Geo-Country-Name: country_name
            X-Geo-Continent: continent
            X-Geo-Continent-Code: continent_code
            X-Geo-Region: region
            X-Geo-City: city
            X-Geo-Isp: isp
            X-Geo-Domain: domain
            X-Geo-Asn: asn
          logStatusDetailHeader: X-Geoblock-Decision
```

Every mapped header is written. A missing field is the string `null`. Country on a private IP is `PRIVATE`.

Which fields are populated depends on the database you load (country-only LITE vs city/ISP/ASN packages). Map only the keys your file actually has if you want to avoid `null` columns.

| Header | Source |
| --- | --- |
| `requestHeaderEnrich` | Geo fields on the request |
| `countryHeader` | ISO country (default `X-IPCountry`). Lookup writes it; block reads it |
| `logStatusDetailHeader` | `pass:{reason}` or `block:{reason}` |

## Optional geoblock

Set `mode` to choose lookup, block, or both. Empty `mode` is `enrichandblock`. `enrich` and `enrichandblock` open the GeoIP database; `block` does not. `disabled` is pass-through with no database.

| `mode` | Lookup + headers | Allow / block |
| --- | --- | --- |
| `enrich` | yes | no |
| `block` | no | yes (reads `countryHeader`) |
| `enrichandblock` | yes | yes |
| `disabled` | no | no |

To share one database and token across routes, put `mode: enrich` on a shared middleware and `mode: block` (country lists only) on each route. **Chain enrich before block.** If `block` runs first, `countryHeader` is missing and `banIfError` applies.

```yaml
http:
  middlewares:
    geo-enrich:
      plugin:
        geoblock:
          mode: enrich
          countryHeader: X-IPCountry
          ipHeaders:
            - x-forwarded-for
            - x-real-ip
          # databaseSources / auto-update live here
    geo-block-us:
      plugin:
        geoblock:
          mode: block
          countryHeader: X-IPCountry
          ipHeaders:
            - x-forwarded-for
            - x-real-ip
          allowedCountries:
            - US
          defaultAllow: false
  routers:
    app:
      rule: Host(`app.example`)
      middlewares:
        - geo-enrich
        - geo-block-us
```

You can also keep a single middleware with `mode: enrichandblock` and the country lists on that same block.

## Installation

**Traefik v3.5.0 or later** is required, and **unsafe operations must be enabled** for this plugin. Traefik may label that setting "unsafe"; it is a Yaegi sandbox flag the IP2Location reader needs, not a security warning about the plugin.

Install [locally](https://traefik.io/blog/using-private-plugins-in-traefik-proxy-2-5/) or from the [Traefik Plugin catalog](https://plugins.traefik.io/plugins).

### Plugin catalog

```yaml
experimental:
  plugins:
    geoblock:
      moduleName: github.com/david-garcia-garcia/traefik-geoblock
      version: v1.0.1
      settings:
        useunsafe: true
```

### Local plugin

```yaml
experimental:
  localPlugins:
    geoblock:
      moduleName: github.com/david-garcia-garcia/traefik-geoblock
      settings:
        useunsafe: true
```

Clone the plugin into the Traefik plugins tree:

```dockerfile
RUN mkdir -p /plugins-local/src/github.com/david-garcia-garcia
RUN git clone https://github.com/david-garcia-garcia/traefik-geoblock \
    /plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock \
    --branch v1.0.1 --single-branch
```

Set `TRAEFIK_PLUGIN_GEOBLOCK_PATH` to that plugin root (the local clone, or the catalog unpack such as `/plugins-storage/sources/github.com/david-garcia-garcia/traefik-geoblock`). Traefik's process working directory is not the plugin tree. The plugin opens `{that dir}/seeds/<filename>` or `{that dir}/<filename>` — it does not walk the tree. `geoblockban.html` is at the plugin root, not under `seeds/`. If the env is unset, logs say it must be the plugin root. If it is set and those exact files are missing, logs say the env is probably not the plugin root. Without this variable, empty seed paths fail unless a dated file is already on an auto-update volume.

```bash
# Docker Compose
environment:
  - TRAEFIK_PLUGIN_GEOBLOCK_PATH=/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock

# Docker run
docker run -e TRAEFIK_PLUGIN_GEOBLOCK_PATH=/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock traefik:latest
```

## GeoIP database

Lookups are local. There is no outbound GeoIP API on the request path. Enable **auto-update** and point `databaseAutoUpdateDir` at a persistent volume so the file stays current and hot-swaps without restarting Traefik.

An empty catalog opens the bundled IP2Location LITE country database (`default_ip2location`). Enable another source and set `default_ip2location.enabled: false` if you do not want both. Several enabled rows merge (first non-empty field wins).

```yaml
databaseAutoUpdateDir: "/data/geoblock"
databaseSources:
  default_ip2location:
    enabled: false
  litezip:
    url: "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP"
    databaseType: bin
    archive: zip
    defaultFile: IP2LOCATION-LITE-DB1.IPV6.BIN
    fieldsPreconfigured: ip2location_lite
  # paid:
  #   url: "https://www.ip2location.com/download?token=YOUR_TOKEN&file=DB8BINIPV6"
  #   databaseType: bin
  #   archive: zip
  #   fieldsPreconfigured: ip2location_db8
  # lite:
  #   url: "https://ipinfo.io/data/ipinfo_lite.mmdb?token=YOUR_TOKEN"
  #   databaseType: mmdb
  #   archive: none
  #   fieldsPreconfigured: ipinfo_lite
  # geolite:
  #   url: "https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz"
  #   databaseType: mmdb
  #   archive: tar.gz
  #   fieldsPreconfigured: maxmind_country
  #   headers:
  #     Authorization: "Basic YOUR_BASE64_ACCOUNTID_LICENSEKEY"
```

Each enabled row needs `databaseType` (`bin` or `mmdb`) and either `fieldsPreconfigured` or a `fields` map — not both. Token and permalink URLs with no file extension need `archive` (`zip`, `tar.gz`, or `none`).

Reserved catalog keys are inserted when missing. An operator-defined reserved key is kept.

| Key | Inserted as | Type / archive | `fieldsPreconfigured` | Bundled `defaultFile` | URL |
| --- | --- | --- | --- | --- | --- |
| `default_ip2location` | enabled | `bin` / `zip` | `ip2location_lite` | `IP2LOCATION-LITE-DB1.IPV6.BIN` | `https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP` |
| `default_ipinfo` | disabled | `mmdb` | `ipinfo_lite` | `ipinfo_lite.mmdb` | add `https://ipinfo.io/data/ipinfo_lite.mmdb?token=YOUR_TOKEN` to keep current |
| `default_maxmind` | disabled | `mmdb` | `maxmind_country` | `GeoIP2-Country-Test.mmdb` (official dummy Country fixture) | official GeoLite: operator row below |
| `default_geolite` | disabled | `mmdb` / `none` | `maxmind_country` | | unofficial `https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb` |

Token and account downloads are operator rows (any key name). Set `archive` on permalinks that have no path extension.

| Example key | Type / archive | `fieldsPreconfigured` | URL |
| --- | --- | --- | --- |
| `paid` | `bin` / `zip` | `ip2location_db8` | `https://www.ip2location.com/download?token=YOUR_TOKEN&file=DB8BINIPV6` |
| `asnlite` | `bin` / `zip` | `ip2location_asn` | `https://www.ip2location.com/download?token=YOUR_TOKEN&file=DBASNLITEBINIPV6` |
| `lite` | `mmdb` / `none` | `ipinfo_lite` | `https://ipinfo.io/data/ipinfo_lite.mmdb?token=YOUR_TOKEN` |
| `geolite` | `mmdb` / `tar.gz` | `maxmind_country` (or `maxmind_city` / `maxmind_asn`) | `https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz` plus `headers.Authorization: Basic …` |

### `fieldsPreconfigured`

`fieldsPreconfigured` names a vendor column map. Record keys you can map in `requestHeaderEnrich` are `country`, `country_name`, `continent`, `continent_code`, `region`, `city`, `isp`, `domain`, `asn`. Extra BIN columns (coords, ZIP, timezone) stay unused.

MMDB presets: `ipinfo_lite`, `ipinfo_core`, `ipinfo_plus`, `maxmind_country`, `maxmind_city`, `maxmind_asn`, plus `geolite2_*` / `geoip2_*` aliases. `fields` is the same map written by hand (`country.iso_code: country`).

IP2Location IPv6 BIN uses one token URL and `archive: zip`. Put the package you own in `file=`. Codes are official download-client names (`DB8BINIPV6`, `DB1LITEBINIPV6`, …), not ZIP filenames. `ip2location` is the same map as `ip2location_db8`. `ip2location_lite` is the same map as `ip2location_db1` / `ip2location_lite_db1`.

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

LITE IPv6 BIN is the same token URL with `DBnLITEBINIPV6`. Free LITE DB1 (no token) is the reserved `default_ip2location` row.

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

ASN LITE is a `bin` row with `fieldsPreconfigured: ip2location_asn`. That package is a token download: set `path` or let auto-update write a dated file.

IP2Location LITE DB1 is country-only; region/city/isp/domain need DB8 or richer. IPinfo Lite fills country, country_name, continent, continent_code, isp (`as_name`), domain (`as_domain`), and asn (`AS15169` form); region and city stay empty.

A custom `fields` map (do not set `fieldsPreconfigured` on the same row):

```yaml
custom:
  databaseType: mmdb
  fields:
    country_code: country
    country.iso_code: country
    autonomous_system_number:
      key: asn
      type: uint32
```

### Which file is used

`databaseSources.<name>.path` is the seed / fallback, not the live copy once a dated file is stored in `databaseAutoUpdateDir`. For each enabled source the plugin picks a file in this order:

1. **`databaseAutoUpdateDir`** — newest dated file for that catalog key (`YYYYMMDD_<catalogKey>.BIN` or `.mmdb`) when the dir is set.
2. **`path` on that source** — if it is an existing file (full path you mounted or copied).
3. **Bundled database** — `seeds/<defaultFile>` under the plugin install, found via `TRAEFIK_PLUGIN_GEOBLOCK_PATH`. Do not put that relative name in `path`.

If none of those exist, plugin creation fails. An ASN LITE row may omit both `path` and `defaultFile` (token download only).

Set `databaseAutoUpdateDir` when a bound entry has a URL. Prefer a persistent volume that survives container replace. If the dir is empty, the plugin WARNs and writes dated files under the process temp dir (`traefik-geoblock`). That path is wiped on container replace — do not rely on `/tmp` in production. After a restart with an empty dir the plugin falls back to seed/`path` and downloads again.

```yaml
databaseAutoUpdateDir: "/data/geoblock"
#   volumes:
#     - geoblock-db:/data/geoblock
```

### Network for auto-update

For automatic updates, allow outbound HTTPS to the vendor you configured:

- `download.ip2location.com` — IP2Location LITE DB1 (no token)
- `www.ip2location.com` — IP2Location token downloads (paid packages and ASN LITE)
- `ipinfo.io` — IPinfo Lite MMDB (token; the response redirects to IPinfo’s CDN)
- `download.maxmind.com` — MaxMind GeoLite2/GeoIP2 permalink (`accountId:licenseKey`; the response redirects)

If `databaseSources` is empty, no external network is required after the bundled seed is on disk.

## Configuration reference

```yaml
http:
  middlewares:
    geoblock:
      plugin:
        geoblock:
          mode: enrichandblock
          defaultAllow: false
          allowPrivate: true
          banIfError: true
          disallowedStatusCode: 403

          allowedCountries:
            - US
            - CA
            - GB
          blockedCountries:
            - RU
            - CN

          allowedIPBlocks:
            - "10.0.0.0/8"
            - "2001:db8::/32"
          blockedIPBlocks:
            - "203.0.113.0/24"
          # Directory of .txt files (one CIDR per line, # comments). Scanned
          # recursively at plugin start. Useful for shared ConfigMaps.
          # File changes need a plugin restart.
          # allowedIPBlocksDir: "/data/allowed-ips/"
          # blockedIPBlocksDir: "/data/blocked-ips/"

          ipHeaders:
            - x-forwarded-for
            - x-real-ip
            # - cf-connecting-ip          # Cloudflare
            # - remoteAddress             # SYNTHETIC: req.RemoteAddr
          ipHeaderStrategy: CheckAll   # CheckAll | CheckFirst | CheckFirstNonePrivate
          ignoreVerbs:
            - OPTIONS
            - HEAD
          includedPathsRegex: ""
          excludedPathsRegex: ""
          bypassHeaders:
            X-Internal-Request: "true"

          countryHeader: X-IPCountry
          requestHeaderEnrich:
            X-Geo-Country: country
            X-Geo-City: city
            X-Geo-Asn: asn
          logStatusDetailHeader: X-Geoblock-Decision

          databaseAutoUpdateDir: "/data/geoblock"
          databaseSources:
            default_ip2location:
              enabled: true

          logLevel: info                 # trace | debug | info | warn | error
          logFormat: json                # json | text
          # Plugin logs go to stdout (Traefik process log).
          # banHtmlFilePath: "/path/to/geoblockban.html"   # {{.IP}} {{.Country}}
          # With ban HTML, use a real status (not 204).
```

### Client IP (`ipHeaders`)

`ipHeaders` cannot be empty. Header order matters: headers are read in list order; within a header, IPs are left-to-right (leftmost is usually the original client). Duplicates are dropped, first occurrence kept.

`remoteAddress` is a synthetic name for `req.RemoteAddr` (the direct connection). Use it when you also need the hop that connected to Traefik:

```yaml
ipHeaders:
  - x-forwarded-for
  - remoteAddress
```

| `ipHeaderStrategy` | Which IPs are evaluated |
| --- | --- |
| `CheckAll` | Every IP found (default) |
| `CheckFirst` | Only the first IP |
| `CheckFirstNonePrivate` | First public IP; if none, first private IP |

Country allow/block uses the single `countryHeader` value (first public country written). `CheckAll` still applies CIDR and private rules to every selected IP. To choose which hop’s country is written, use `CheckFirst` / `CheckFirstNonePrivate`. To allow or deny a later hop by address, use CIDR lists or omit that hop from `ipHeaders`.

On lookup modes, `countryHeader` starts as `PRIVATE` and is overwritten by the first real country.

### Path include / exclude

`includedPathsRegex` and `excludedPathsRegex` are one Go RE2 regex each, matched against `{host}{path}` (for example `example.com/api/users`). Host omits the port for 80/443. Path starts with `/` and has no query string. Empty is unset (no effect). Include runs first; exclude still wins after a match. Requests that skip blocking still get enrichment.

A public URL match is not a secret — anyone who can guess the path skips blocking. For health checks, `bypassHeaders` is stronger.

```yaml
includedPathsRegex: "^[^/]*/secure/.*"
excludedPathsRegex: "^[^/]*/(health|ready|live)$"
```

| Example | Matches |
| --- | --- |
| `^[^/]*/secure/.*` | `/secure/*` on any host |
| `^app\\.example\\.com/admin/.*` | `/admin/*` on that host |
| `^[^/]*/health$` | `/health` on any host |
| `^api\\.example\\.com/.*` | all paths on `api.example.com` |

`ignoreVerbs` is case-insensitive. Those methods skip blocking and still get enrichment.

### Ban page

`banHtmlFilePath` is a full path, or empty (status only). If the path is missing, the plugin opens `$TRAEFIK_PLUGIN_GEOBLOCK_PATH/geoblockban.html`. Template variables: `{{.IP}}` and `{{.Country}}`. Do not pair HTML with status `204`.

### Processing order

1. `mode: disabled` → pass through
2. `bypassHeaders`
3. `ignoreVerbs` → skip blocking, keep enrichment
4. `includedPathsRegex` set and no match → skip blocking, keep enrichment
5. `excludedPathsRegex` match → skip blocking, keep enrichment (still wins after include)
6. Read IPs from `ipHeaders` in list order
7. Apply `ipHeaderStrategy`
8. `enrich` / `enrichandblock` → lookup and write `countryHeader` + `requestHeaderEnrich`
9. `block` / `enrichandblock` → private and CIDR rules per selected IP, then country from `countryHeader`
10. `defaultAllow` if no country rule matched

| Setting | Role |
| --- | --- |
| `mode` | `disabled` \| `enrich` \| `block` \| `enrichandblock` (empty = `enrichandblock`) |
| `requestHeaderEnrich` | Header name → geo key |
| `countryHeader` | Country bridge between enrich and block (default `X-IPCountry`) |
| `logStatusDetailHeader` | `pass:{reason}` / `block:{reason}` on the request |
| `allowedCountries` / `blockedCountries` | ISO 3166-1 alpha-2 |
| `defaultAllow` | When no country or CIDR rule matches |
| `allowPrivate` | RFC 1918 / loopback (country `PRIVATE`) |
| `allowedIPBlocks` / `blockedIPBlocks` | CIDR allow / deny (more specific prefix wins) |
| `allowedIPBlocksDir` / `blockedIPBlocksDir` | Shared `.txt` CIDR files, loaded at start |
| `ipHeaders` | Where to read client IPs (`remoteAddress` is the direct connection) |
| `ipHeaderStrategy` | Which hop to evaluate when several IPs are present |
| `ignoreVerbs` / path regex / `bypassHeaders` | Skip blocking; enrichment still runs |
| `banIfError` / `disallowedStatusCode` / `banHtmlFilePath` | Lookup failure and ban response |
| `databaseSources` / `databaseAutoUpdateDir` | Local files and optional download |
| `logLevel` / `logFormat` | Plugin logs on Traefik stdout |

---

📄 Apache License 2.0 — see [LICENSE](LICENSE).

🌍 This project includes IP2Location LITE data available from [`lite.ip2location.com`](https://lite.ip2location.com/database/ip-country).

🌍 This project includes [IPinfo Lite](https://ipinfo.io) data (CC-BY-SA 4.0). IP address data powered by [IPinfo](https://ipinfo.io).

🌍 This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com.
