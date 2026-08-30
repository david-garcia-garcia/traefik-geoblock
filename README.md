# 🌍 Traefik Geoblock

[![Build Status](https://github.com/david-garcia-garcia/traefik-geoblock/actions/workflows/ci.yml/badge.svg)](https://github.com/david-garcia-garcia/traefik-geoblock/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/david-garcia-garcia/traefik-geoblock)](https://goreportcard.com/report/github.com/david-garcia-garcia/traefik-geoblock)
[![Latest GitHub release](https://img.shields.io/github/v/release/david-garcia-garcia/traefik-geoblock?sort=semver)](https://github.com/david-garcia-garcia/traefik-geoblock/releases/latest)
[![License](https://img.shields.io/badge/license-Apache%202.0-brightgreen.svg)](LICENSE)  

A Traefik middleware that looks up the client IP in a **local** GeoIP database (IP2Location or IPinfo) and writes country, city, ASN, and related fields onto the **request**. Use that to make **Traefik access logs and your applications geo-aware** — no per-request GeoIP API. The same lookup can **allow or block** by country or CIDR when you want it to.

> [!TIP]
> 
> **Traefik Security**
> 
> The basic middlewares you need to secure your Traefik ingress:
> 
> 🌍 **Geoblock**: [david-garcia-garcia/traefik-geoblock](https://github.com/david-garcia-garcia/traefik-geoblock) - Geo-enrich requests for logs and backends; optionally allow or block by country  
> 🛡️ **CrowdSec**: [maxlerebourg/crowdsec-bouncer-traefik-plugin](https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin) - Real-time threat intelligence and automated blocking  
> 🔒 **ModSecurity CRS**: [david-garcia-garcia/traefik-modsecurity](https://github.com/david-garcia-garcia/traefik-modsecurity) - Web Application Firewall with OWASP Core Rule Set  
> 🚦 **Ratelimit**: [Traefik Rate Limit](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/ratelimit/) - Control request rates and prevent abuse

> [!WARNING]
>
> **You should not run middlewares as Yaegi plugins in production.**
>
> Traefik's default plugin system runs plugins via [Yaegi](https://github.com/traefik/yaegi) (a Go interpreter) at runtime. Middlewares run on every request, so they sit on the hot path. Using an interpreter for that workload has concrete drawbacks related to memory management, CPU usage and observability (see [feat: improve pprof experience by adding wrappers to interpreted functions by david-garcia-garcia · Pull Request #1712 · traefik/yaegi](https://github.com/traefik/yaegi/pull/1712))
>
> For production deployments where middlewares handle substantial traffic, use a Traefik build that **compiles those middlewares into the binary** instead of loading them as Yaegi plugins such as in [david-garcia-garcia/traefik-with-plugins: Traefik container with preloaded plugins in it](https://github.com/david-garcia-garcia/traefik-with-plugins)
>
> **For more details and discussion, read [Traefik issue #12213](https://github.com/traefik/traefik/issues/12213) in the Traefik issue queue.**

## Performance & Scalability

**Designed for high-performance production environments:**

- **No per-request GeoIP API** — lookups use a local IP2Location BIN, IPinfo MMDB, or MaxMind GeoIP2 MMDB
- **Minimal memory footprint** — no application-level cache; the database format is read in place
- **Offline after load** — no outbound call unless you enable auto-update
- **Hot-swappable database updates** — new files load without restarting Traefik

This architecture ensures consistent response times and eliminates external service bottlenecks, making it ideal for high-traffic environments and air-gapped deployments.

**Expected throughput** (`go test -bench=BenchmarkPlugin -benchmem` on a local Intel Core Ultra 7 265K). `Lookup` always reads the full geo row (`Get_all` on IP2Location). `ServeHTTP` reuse is the Traefik path (request/recorder reused). Country-only vs full `requestHeaderEnrich` on the same BIN is the same lookup; extra headers are cheap. MaxMind benches use the committed dummy `GeoIP2-Country-Test.mmdb` and dummy IP `81.2.69.142` (not 8.8.8.8). A live GeoLite2-Country file is much larger and will be slower.

| Database | `Lookup` | `ServeHTTP` reuse | Typical cost |
| --- | --- | --- | --- |
| LITE DB1 (country) | ~148k ops/s | ~144k ops/s | ~6.7–7.0 µs, 4–5 allocs, ~540 B |
| LITE DB1 + ASN LITE | ~48k ops/s | ~52k ops/s | ~19–21 µs, 9–10 allocs, ~1200 B |
| Paid DB8 (country only) | ~37k ops/s | ~46k ops/s | ~22–27 µs, 13–14 allocs, ~1840 B |
| Paid DB8 + full enrich (country/region/city/isp/domain) | ~46k ops/s | ~45k ops/s | ~22 µs, 20 allocs, 1944 B |
| Paid DB8 + ASN LITE (full enrich) | ~27k ops/s | ~22k ops/s | ~38–45 µs, 18–26 allocs, ~2500 B |
| MaxMind dummy Country | ~1.7M ops/s | ~1.3M ops/s | ~0.60–0.80 µs, 5–6 allocs, ~140–155 B |

## Geo enrichment and observability

Every request that reaches the plugin gets a local GeoIP lookup. The result is written onto the **request** (`requestHeaderEnrich`), not the HTTP response:

- **Applications** behind Traefik see the same headers (`X-Geo-Country`, city, ASN, …) and can branch on them without calling a GeoIP service.
- **Traefik access logs** can `keep` those names so every line is geo-aware (dashboards, SIEM, compliance).
- Headers are **not** copied onto the response, so browsers and other clients do not see them.

Use `mode` to choose lookup, block, or both. Empty `mode` is `disabled` (pass through, no database). `enrich` and `enrichandblock` open the GeoIP database; `block` does not. Lookup writes the country to `countryHeader` (default `X-IPCountry`); the block stage reads that same header.

To share one download/token config across routes, put `mode: enrich` on a shared middleware and `mode: block` (country lists only) on each route. Chain enrich before block. `block` must not run first or `countryHeader` is missing and `banIfError` applies.

> **Recommended**: map the fields you need with `requestHeaderEnrich`, and set `logStatusDetailHeader` if you also want the allow/block reason (`pass:{reason}` / `block:{reason}`). Keep those header names in Traefik access logs.

### Available Headers

| Config Setting | Purpose | Example Values |
|----------------|---------|----------------|
| `requestHeaderEnrich` | Geo metadata on the request. Every mapped header is written. Missing values are `null`. Country on a private IP is `PRIVATE`. | `X-Geo-Country: US`, `X-Geo-Region: null` |
| `countryHeader` | Lookup writes the country here; block reads it. Omitted value is `X-IPCountry`. | `US`, `DE`, `PRIVATE` |
| `logStatusDetailHeader` | Decision with reason | `pass:allowed_country`, `block:blocked_country` |

### logStatusDetailHeader Values

Detailed status with format `{action}:{reason}`:

**Bypass reasons (blocking skipped, checked first):**

These are evaluated before geo-blocking rules. If any match, the request passes without geo evaluation:

| Value | Description | Example Scenario |
|-------|-------------|------------------|
| `pass:ignore_verb` | HTTP method is in `ignoreVerbs` list | OPTIONS request for CORS preflight |
| `pass:not_included_regex` | `includedPathsRegex` is set and the request did not match | Public path `/docs` when include is `/secure/.*` |
| `pass:excluded_regex` | Request matched `excludedPathsRegex` pattern | Health check endpoint `/health` |
| `pass:bypass_header` | Request had matching `bypassHeaders` header/value | Internal service with secret header |

**Geo-rule pass reasons (request allowed by geo-blocking rules):**

These indicate why the geo-blocking logic allowed the request:

| Value | Description | Example Scenario |
|-------|-------------|------------------|
| `pass:allow_private` | IP is private/internal (RFC 1918) and `allowPrivate` is true | Request from `192.168.1.100` |
| `pass:allowed_ip_block` | IP matched a CIDR range in `allowedIPBlocks` or `allowedIPBlocksDir` | Trusted partner IP `203.0.113.50` |
| `pass:allowed_country` | Country code is in `allowedCountries` list | US user when `allowedCountries: ["US"]` |
| `pass:default_allow` | No rules matched and `defaultAllow` is true | Unknown country with permissive config |
| `pass:none` | No IP addresses found to evaluate | Misconfigured `ipHeaders` |

**Block reasons (request denied):**

| Value | Description | Example Scenario |
|-------|-------------|------------------|
| `block:allow_private` | IP is private/internal but `allowPrivate` is false | Internal IP blocked by strict config |
| `block:blocked_ip_block` | IP matched a CIDR range in `blockedIPBlocks` or `blockedIPBlocksDir` | Known bad actor IP range |
| `block:blocked_country` | Country code is in `blockedCountries` list | Request from blocked region |
| `block:default_allow` | No rules matched and `defaultAllow` is false | Unknown country with strict config |
| `block:error` | IP lookup failed and `banIfError` is true | Database lookup failure |

### Traefik Access Log Configuration

```yaml
accessLog:
  filePath: "/var/log/traefik/access.log"
  format: json
  fields:
    headers:
      names:
        X-Geo-Country: keep
        X-Geo-City: keep
        X-Geo-Asn: keep
        X-Geoblock-Decision: keep
```

Access logs then carry geo fields and the decision reason. The backend already received the same request headers. Clients do not.

## Features

- Local GeoIP lookup (IP2Location or IPinfo) on every request — no outbound GeoIP API
- **Geo enrichment** (`requestHeaderEnrich`): country, country name, continent, region, city, ISP, domain, ASN on the request for access logs and backend apps
- Optional **geoblock**: allow or deny by ISO 3166-1 alpha-2 country and by CIDR (inline or directory files)
- Optional bypass using custom headers
- Configurable handling of private/internal networks
- Customizable error responses when a request is blocked
- Hot-swap database updates without restarting Traefik
- Path include/exclude via regex — skip blocking on those paths, keep enrichment

## Installation

> ⚠️ IMPORTANT REQUIREMENTS
>
> - **Traefik v3.5.0 or later** is required (unsafe support was introduced in v3.5.0)
>
> - **Unsafe operations must be enabled** in Traefik configuration
>
>   ### Why "Unsafe" Mode is Required
>
>   Traefik may display this plugin as "unsafe", which can be misleading. **This does not mean the plugin is dangerous or insecure.**
>
>   **What "unsafe" actually means:**
>
>   Traefik plugins run inside [Yaegi](https://github.com/traefik/yaegi), a Go interpreter that sandboxes plugin code for security. By default, Yaegi restricts access to Go's [`unsafe`](https://pkg.go.dev/unsafe) package - a low-level Go standard library package used for memory operations and performance optimizations.
>
>   **Why this plugin needs it:**
>
>   This plugin depends on the [ip2location-go](https://github.com/ip2location/ip2location-go) library, which uses `unsafe.Pointer` for efficient byte-to-string conversions when reading the binary database file. This is a common Go performance optimization pattern that avoids unnecessary memory allocations during IP lookups.
>
>   ```go
>   // Example from ip2location library - efficient string conversion
>   return *(*string)(unsafe.Pointer(&b))
>   ```
>

It is possible to install the [plugin locally](https://traefik.io/blog/using-private-plugins-in-traefik-proxy-2-5/) or to install it through [Traefik Plugins]([Plugins](https://plugins.traefik.io/plugins)).

### Local Plugin Installation

Create or modify your Traefik static configuration

```yaml
experimental:
  localPlugins:
    geoblock:
      moduleName: github.com/david-garcia-garcia/traefik-geoblock
      settins:
        useunsafe: true
  # REQUIRED: Enable unsafe operations for this plugin
  plugins:
    geoblock:
      settings:
        useunsafe: true
```

You should clone the plugin into the container, i.e

```dockerfile
# Create the directory for the plugins
RUN set -eux; \
    mkdir -p /plugins-local/src/github.com/david-garcia-garcia

RUN set -eux && git clone https://github.com/david-garcia-garcia/traefik-geoblock /plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock --branch v1.0.1 --single-branch
```

### Traefik Plugin Registry Installation

Add to your Traefik static configuration:

```yaml
experimental:
  plugins:
    geoblock:
      moduleName: github.com/david-garcia-garcia/traefik-geoblock
      version: v1.0.1
      # REQUIRED: Enable unsafe operations for this plugin
      settings:
        useunsafe: true
```

## GeoIP database configuration and updates

The plugin looks up IPs from **enabled** `databaseSources` rows. Each row sets `vendor` (`ip2location`, `ipinfo`, or `maxmind`) and optional `defaultFile` (basename under `seeds/`). Optional `fields` is an allowlist of normalized Record keys (`country`, `asn`, …). Empty `fields` keeps every key that vendor’s code map can fill. Omitted `enabled` means on. Config has no `databaseProvider` and no `*_source_*` pointers.

| Vendor | Format | Shipped catalog row | Bundled `defaultFile` |
| --- | --- | --- | --- |
| `ip2location` | `bin` | `default_ip2location` (**enabled**) — free LITE ZIP URL | `IP2LOCATION-LITE-DB1.IPV6.BIN` |
| `ipinfo` | `mmdb` | `default_ipinfo` (disabled) | `ipinfo_lite.mmdb` |
| `maxmind` | `mmdb` | `default_maxmind` (disabled dummy Country); `default_geolite` (disabled unofficial GET) | `GeoIP2-Country-Test.mmdb` on `default_maxmind` |

IP2Location ASN LITE is the same `vendor: ip2location` (`bin`). Set `fields: [asn]` so the row calls `Get_asn` and does not treat the file as a geo BIN. There is no shipped ASN seed (token download).

Empty lookup config opens only `default_ip2location`. Enable another row and set `default_ip2location.enabled: false` if you do not want both. Several enabled rows merge: first non-empty field wins, in lexicographic catalog-key order.

**Finding the bundled files.** Traefik does not put the plugin tree on the process working directory. Set `TRAEFIK_PLUGIN_GEOBLOCK_PATH` to the plugin root (the local clone, or the registry unpack such as `/plugins-storage/sources/github.com/david-garcia-garcia/traefik-geoblock`). The plugin opens `{that dir}/seeds/<filename>` or `{that dir}/<filename>` — it does not walk the tree. If the env is unset, logs say it must be the plugin root. If it is set and those exact files are missing, logs say the env is probably not the plugin root. Without this variable, empty seed paths fail unless a dated file is already on an auto-update volume.

**Recommendation: configure auto-update** and point the auto-update directory at a persistent volume. The bundled snapshots (and any static seed file you copy in) go stale. Auto-update downloads a current database, stores a dated copy, and hot-swaps it without restarting Traefik.

How auto-update works:

- The plugin always inserts reserved catalog keys unless you already defined them:
  - `default_ip2location` — enabled; `vendor: ip2location`; free IP2Location country LITE ZIP; `defaultFile` `IP2LOCATION-LITE-DB1.IPV6.BIN`.
  - `default_ipinfo` — disabled; `vendor: ipinfo`; `defaultFile` `ipinfo_lite.mmdb`.
  - `default_maxmind` — disabled; `vendor: maxmind`; `defaultFile` `GeoIP2-Country-Test.mmdb` (official dummy Country fixture, not a live GeoLite file).
  - `default_geolite` — disabled; unofficial [P3TERX GeoLite.mmdb](https://github.com/P3TERX/GeoLite.mmdb/tree/download) Country GET. Official GeoLite still needs an account. Operator-defined reserved keys are kept.
- Add named entries under `databaseSources` (`vendor`, `url`, `databaseType`, `archive`, optional `headers`, `path`, `defaultFile`, `fields`, `enabled`). Keys are operator-chosen except the reserved names above. A token may live in the URL query (`?token=`). `path` is the operator seed **file** (use a full path). It is not the bundled `seeds/` copy. `databaseType` is `bin` or `mmdb` and must match `vendor`. Set `archive` to `none`, `zip`, or `tar.gz`. Empty `archive` is inferred from the URL path. Official IP2Location token and MaxMind permalink URLs have no path extension — set `archive` on those entries. Examples:

  ```yaml
  databaseSources:
    # Free IP2Location country LITE (no token). Official public ZIP.
    litezip:
      vendor: ip2location
      url: "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP"
      databaseType: bin
      archive: zip
      defaultFile: IP2LOCATION-LITE-DB1.IPV6.BIN
    # IP2Location token download (paid package or ASN LITE). No path extension — set archive.
    paid:
      vendor: ip2location
      url: "https://www.ip2location.com/download?token=YOUR_TOKEN&file=DB8BINIPV6"
      databaseType: bin
      archive: zip
    asnlite:
      vendor: ip2location
      fields: [asn]
      url: "https://www.ip2location.com/download?token=YOUR_TOKEN&file=DBASNLITEBINIPV6"
      databaseType: bin
      archive: zip
    # IPinfo Lite MMDB (token in the query).
    lite:
      vendor: ipinfo
      url: "https://ipinfo.io/data/ipinfo_lite.mmdb?token=YOUR_TOKEN"
      databaseType: mmdb
      archive: none
    # MaxMind GeoLite2/GeoIP2 permalink (Basic auth). No path extension — set archive.
    geolite:
      vendor: maxmind
      url: "https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz"
      databaseType: mmdb
      archive: tar.gz
      headers:
        Authorization: "Basic YOUR_BASE64_ACCOUNTID_LICENSEKEY"
  ```

- Enable the rows you want. Omitted `enabled` is on. Disable `default_ip2location` when another country source should win (or be the only one).

  ```yaml
  databaseSources:
    default_ip2location:
      enabled: false
    litezip:
      vendor: ip2location
      # ...
    asnlite:
      vendor: ip2location
      fields: [asn]
      # ...
  ```
- Set `databaseAutoUpdateDir` when a bound entry has a URL. Prefer durable storage: a persistent volume that survives container restarts **and** replacement. If the dir is empty, the plugin WARNs and writes dated files under the process temp dir (`traefik-geoblock`). That path is wiped on container replace. Do not rely on `/tmp` in production. If the directory is empty after a restart the plugin falls back to seed/`path` and will download again.

  ```yaml
  databaseAutoUpdateDir: "/data/geoblock"
  # Docker: mount a named volume or bind mount at that path, e.g.
  #   volumes:
  #     - geoblock-db:/data/geoblock
  ```

`databaseSources.<name>.path` is the seed / fallback, not the live copy once a dated file is stored in `databaseAutoUpdateDir`. The plugin picks a file for each bound source in this order:

1. **`databaseAutoUpdateDir`** — newest dated file for that `databaseSources` key (when the dir and a pointer are set).
2. **`path` on that source** — if it is an existing file (full path to a file you mounted or copied).
3. **Bundled database** — `seeds/<default filename>` under the plugin install, found via `TRAEFIK_PLUGIN_GEOBLOCK_PATH`. Do not put that relative name in `path`.

If none of those exist, plugin creation fails (except an empty IP2Location ASN pointer and `path`, which means “no ASN”). There is no `*_databaseFilePath` or `databaseFilePath` key.

## Network Requirements

**For automatic database updates to function, ensure your firewall allows outbound HTTPS connections to:**

- `download.ip2location.com` — IP2Location LITE DB1 (no token)
- `www.ip2location.com` — IP2Location token downloads (paid packages and ASN LITE)
- `ipinfo.io` — IPinfo Lite MMDB (token required; the response redirects to IPinfo’s download CDN)
- `download.maxmind.com` — MaxMind GeoLite2/GeoIP2 permalink (`accountId:licenseKey`; the response redirects to MaxMind’s download host)

> **Note:** If `databaseSources` is empty, no external network access is required and the plugin operates entirely offline. Paste the official IP2Location lite ZIP URL if you want LITE without a token. ASN LITE, IPinfo, and MaxMind downloads need the official URL (and headers) you own.

## Testing and development

You can spin up a fully working environment with docker compose:

```powershell
docker compose up --build
```

The codebase includes a full set of integration and unit tests:
```powershell
# Run unit tests
go test

# Throughput gates (also run in CI; skipped with -short)
go test -run TestThroughput -v

# Compare lookup/request cost before and after a change
go test -bench=BenchmarkPlugin -benchmem

# MaxMind dummy fixture only (uses 81.2.69.142, not 8.8.8.8)
go test -run '^$' -bench=MaxMind -benchmem

# Run integration tests
.\Test-Integration.ps1
```

Token-protected downloads (IP2Location paid/LITE-with-token, ASN LITE, IPinfo Lite) are optional and local-only. Copy `.env.example` to `.env`, set the tokens, then run `.\Test-Integration.ps1`. Compose starts profile `local-tokens` (`/tokendb`) when `IP2LOCATION_DOWNLOAD_TOKEN` is set. Pester waits for Traefik log lines (`database updated successfully`, `hot-swapped`, `IPinfo database updated`). Cases skip when the matching token is empty. CI does not load `.env`.

```

## Configuration

### Environment Variables

The plugin supports the following environment variable for configuration:

- **`TRAEFIK_PLUGIN_GEOBLOCK_PATH`**: Plugin root. Used when a catalog `path` is empty or missing. Exact files only: `seeds/<bundled name>` then `<bundled name>` at that root (`geoblockban.html` is at the root, not under `seeds/`).

Example usage:
```bash
# Docker Compose
environment:
  - TRAEFIK_PLUGIN_GEOBLOCK_PATH=/data/geoblock

# Docker run
docker run -e TRAEFIK_PLUGIN_GEOBLOCK_PATH=/data/geoblock traefik:latest

# System environment variable
export TRAEFIK_PLUGIN_GEOBLOCK_PATH=/opt/traefik-plugins/geoblock
```

When this environment variable is set, the plugin opens `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN`, `seeds/ipinfo_lite.mmdb`, `seeds/GeoIP2-Country-Test.mmdb`, and `geoblockban.html` at that root if they are not found at the configured path. There is no bundled ASN file; ASN needs catalog `path` or a dated auto-update file. Downloads run only when the matching pointer names a catalog entry with a URL.

### Example Docker Compose Setup

```yaml
version: "3.7"

services:
  traefik:
    image: traefik:v3.5.3  # v3.5.0 or later required
    command:
      # REQUIRED: Enable unsafe operations for geoblock plugin
      - "--experimental.plugins.geoblock.settings.useunsafe=true"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./traefik.yml:/etc/traefik/traefik.yml
      - ./dynamic-config.yml:/etc/traefik/dynamic-config.yml
      - ./seeds/IP2LOCATION-LITE-DB1.IPV6.BIN:/plugins-storage/IP2LOCATION-LITE-DB1.IPV6.BIN
    ports:
      - "80:80"
      - "443:443"

  whoami:
    image: traefik/whoami
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.whoami.rule=Host(`whoami.localhost`)"
      - "traefik.http.routers.whoami.middlewares=geoblock@file"
```

### Dynamic Configuration

```yaml
http:
  middlewares:
    geoblock:
      plugin:
        geoblock:
          #-------------------------------
          # Core Settings
          #-------------------------------
          mode: enrichandblock            # disabled | enrich | block | enrichandblock (empty = disabled)
          defaultAllow: false             # Default behavior when no rules match (false = block)
          
          #-------------------------------
          # Database Configuration
          #-------------------------------
          # Catalog rows: vendor + optional defaultFile / enabled. See Database downloads.
          
          #-------------------------------
          # Country-based Rules (ISO 3166-1 alpha-2 format)
          #-------------------------------
          allowedCountries:               # Whitelist of countries to allow
            - "US"                        # United States
            - "CA"                        # Canada
            - "GB"                        # United Kingdom
          blockedCountries:               # Blacklist of countries to block
            - "RU"                        # Russia
            - "CN"                        # China
            
          #-------------------------------
          # Network Rules
          #-------------------------------
          allowPrivate: true              # Allow requests from private/internal networks (marked as "PRIVATE")
          # This includes RFC 1918 private networks (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
          # and loopback addresses (127.0.0.0/8 for IPv4, ::1 for IPv6)
          allowedIPBlocks:                # CIDR ranges to always allow (highest priority)
            - "192.168.0.0/16"
            - "10.0.0.0/8"
            - "2001:db8::/32"
          blockedIPBlocks:                 # CIDR ranges to always block
            - "203.0.113.0/24"
            # More specific ranges (longer prefix) take precedence
          
          # Directory-based IP blocks (loaded once during plugin initialization)
          # This is useful if you mount configmaps in your traefik plugin
          # so that these will be shared among all Geoip middleware instances
          allowedIPBlocksDir: "/data/allowed-ips/"   # Directory with .txt files containing allowed CIDR blocks
          blockedIPBlocksDir: "/data/blocked-ips/"   # Directory with .txt files containing blocked CIDR blocks
          # All .txt files in the directory are scanned recursively during plugin startup
          # Each .txt file should contain one CIDR block per line (comments with # supported)
          # Note: Changes to files require plugin restart to take effect
          # Example file content:
          #   # AWS IP ranges
          #   172.16.0.0/12
          #   203.0.113.0/24
          
          #-------------------------------
          # IP Extraction Configuration
          #-------------------------------
          ipHeaders:                      # List of headers to check for client IP addresses (required, cannot be empty)
            - "x-forwarded-for"           # Default: check X-Forwarded-For header first
            - "x-real-ip"                 # Default: check X-Real-IP header second
          # Custom examples:
          # - "cf-connecting-ip"          # Cloudflare
          # - "x-client-ip"               # Custom proxy
          # - "remoteAddress"             # SYNTHETIC: Maps to req.RemoteAddr (direct connection IP)
          # 
          # IMPORTANT: Header order matters! IPs are processed in the order headers are defined.
          # Within each header, IPs are processed left-to-right (leftmost = original client IP).
          # Duplicate IPs are automatically removed, preserving the first occurrence.
          #
          # SYNTHETIC HEADERS:
          # - "remoteAddress": Special synthetic header that maps to req.RemoteAddr field
          #   This provides access to the actual network connection's remote address
          #   Useful when you need to check the direct connection IP alongside proxy headers
          #
          # Example configurations:
          # ipHeaders: ["x-forwarded-for", "remoteAddress"]  # Check proxy header first, then direct connection
          # ipHeaders: ["remoteAddress"]                     # Only check direct connection IP
          # ipHeaders: ["remoteAddress", "x-real-ip"]        # Check direct connection first, then proxy header
          
          ipHeaderStrategy: "CheckAll"    # Strategy for processing multiple IP addresses (default: CheckAll)
                                          # Options:
                                          # - "CheckAll": Check all IPs found in headers (original behavior)
                                          # - "CheckFirst": Check only the first IP address found
                                          # - "CheckFirstNonePrivate": Check first non-private IP, fallback to first private IP if no public IPs found
          
          ignoreVerbs:                    # List of HTTP verbs to ignore for blocking (still enriched with GeoIP)
            - "OPTIONS"                   # Common for CORS preflight requests
            - "HEAD"                      # Common for health checks
          # Additional examples:
          # - "TRACE"                     # HTTP TRACE method
          # - "CONNECT"                   # HTTP CONNECT method
          # Note: Verb matching is case-insensitive
          
          #-------------------------------
          # Path Inclusion / Exclusion
          #-------------------------------
          # Both settings are one Go RE2 regex matched against "{host}{path}"
          # (e.g. example.com/api/users). Empty = unset (no effect).
          # Include runs first; exclude still wins after a match.
          # Requests that skip blocking still receive GeoIP enrichment.
          #
          # MATCHING FORMAT: "{host}{path}"
          # - host: The Host header (port omitted for 80/443, may appear otherwise)
          # - path: URL path starting with / (no query string)
          #
          includedPathsRegex: ""
          # WHEN SET: only matching requests are candidates for blocking.
          # Non-matching requests pass (pass:not_included_regex) and are still enriched.
          # Unset/empty: every request is a candidate (same as today).
          # A public URL match is not a secret: anyone who can guess the path
          # skips blocking. For health checks, bypassHeaders is stronger.
          # Examples:
          # - "^[^/]*/secure/.*"                 # only /secure/* on any host
          # - "^app\\.example\\.com/admin/.*"    # only /admin/* on that host
          #
          excludedPathsRegex: "^[^/]*/api/.*"
          # Matching requests skip geoblocking (pass:excluded_regex) even if they
          # also matched includedPathsRegex. Useful for /health inside /secure/*.
          # For health checks, bypassHeaders is more secure (secret header vs public URL).
          #
          # Examples:
          # - "^[^/]*/health$"                   # /health on any domain
          # - "^[^/]*/(health|ready|live)$"      # Health check paths on any domain
          # - "^[^/]*/api/.*"                    # All /api/* paths on any domain
          # - "^api\\.example\\.com/.*"          # All paths on api.example.com
          # - "^internal\\..*/(health|metrics)$" # /health or /metrics on internal.* subdomains
          # - "/health$"                         # /health path (partial match, any domain)
          
          #-------------------------------
          # Bypass Configuration
          #-------------------------------
          bypassHeaders:                  # Headers that skip geoblocking entirely
            X-Internal-Request: "true"
            X-Skip-Geoblock: "1"
            X-Cdn-Auth: "mysupersecretkey"
            
          #-------------------------------
          # Error Handling and ban
          #-------------------------------
          banIfError: true                # Block requests if IP lookup fails
          disallowedStatusCode: 403       # HTTP status code for blocked requests. If you are using banHtmlFilePath make sure to set this to a valid code (such as NOT 204).
          
          banHtmlFilePath: "/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock/geoblockban.html"
          # Can be:
          # - Full path to an existing file: /path/to/geoblockban.html
          # - Empty: returns only status code
          # If the path is missing, the plugin opens
          # $TRAEFIK_PLUGIN_GEOBLOCK_PATH/geoblockban.html (plugin root).
          # Template variables available: {{.IP}} and {{.Country}}
          
          #-------------------------------
          # Logging Configuration
          #-------------------------------
          logLevel: "info"                  # Available: trace, debug, info, warn, error
          # Per-request ServeHTTP logs (bypass, exclude, ignore verb) are trace.
          logFormat: "json"                 # Available: json, text
          # Plugin logs go to stdout (Traefik's process log). There is no file logger.
          # Observe allow/block decisions with logStatusDetailHeader in access logs.

          #-------------------------------
          # Database downloads (all providers)
          #-------------------------------
          # Named catalog. Reserved keys are inserted when missing
          # (default_ip2location enabled; default_ipinfo / default_maxmind /
          # default_geolite disabled). Set vendor on every enabled row.
          # Empty databaseAutoUpdateDir + enabled URL WARNs and uses a temp dir.
          # Dated files: YYYYMMDD_<catalogKey>.BIN or .mmdb.
          databaseAutoUpdateDir: "/data/geoblock"
          databaseSources:
            default_ip2location:
              enabled: false
            litezip:
              vendor: ip2location
              url: "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP"
              databaseType: bin
              archive: zip
              defaultFile: IP2LOCATION-LITE-DB1.IPV6.BIN
            # asnlite:
            #   vendor: ip2location
            #   fields: [asn]
            #   url: "https://www.ip2location.com/download?token=$TOKEN&file=DBASNLITEBINIPV6"
            #   databaseType: bin
            #   archive: zip

          # Optional operator seed (full file path). Used when the auto-update dir
          # has no dated file. Bundled seeds/ use defaultFile + TRAEFIK_PLUGIN_GEOBLOCK_PATH.
          # databaseSources.litezip.path: "/data/geo/IP2LOCATION-LITE-DB1.IPV6.BIN"

          #-------------------------------
          # Request header settings
          #-------------------------------  
          countryHeader: "X-IPCountry"
          # Default X-IPCountry when omitted. Enrich/enrichandblock write the ISO
          # country (or PRIVATE) here. Block/enrichandblock read this header for
          # country allow/block. If requestHeaderEnrich also maps country, it must
          # use this same header name.

          requestHeaderEnrich:
            X-IPCountry: country            # must be the same header as countryHeader
            X-Geo-Country-Name: country_name
            X-Geo-Continent: continent
            X-Geo-Continent-Code: continent_code
            X-Geo-Region: region
            X-Geo-City: city
            X-Geo-Isp: isp
            X-Geo-Domain: domain
            X-Geo-Asn: asn
          # Map request header names to geo metadata keys:
          # country, country_name, continent, continent_code, region, city, isp, domain, asn.
          # The first public IP wins. Every mapped header is written; empty or
          # unavailable fields are the string null (logs and backends need the header present).
          # IP2Location LITE DB1 is country-only; region/city/isp/domain need DB8 or richer.
          # asn from IP2Location ASN LITE needs an enabled row with vendor
          # ip2location and fields: [asn].
          # IPinfo Lite fills country, country_name, continent, continent_code,
          # isp (as_name), domain (as_domain), and asn (AS15169 form).
          # region and city stay empty.
          
          logStatusDetailHeader: "X-Geoblock-Decision"
          # Optional header to add the decision to the REQUEST
          # Format: "pass:{reason}" or "block:{reason}"
          # See the Observability section for all possible values
          # Example access log config: accesslog.fields.headers.names.X-Geoblock-Decision=keep


```

Share one catalog across routes by chaining `enrich` then `block`. Put download and tokens on the shared enrich middleware. Put country lists on each route’s block middleware. Order matters: enrich must run first so `countryHeader` is present.

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
          databaseSources:
            default_ip2location:
              enabled: false
            litezip:
              vendor: ip2location
              url: "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP"
              databaseType: bin
              archive: zip
              defaultFile: IP2LOCATION-LITE-DB1.IPV6.BIN
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

### Processing Order

The plugin processes requests in the following order:

1. If `mode` is `disabled`, pass through
2. Check bypass headers
3. Check if HTTP verb is in ignoreVerbs list (skip blocking but continue enrichment)
4. If includedPathsRegex is set and the request does not match, skip blocking but continue enrichment
5. Check if request matches excludedPathsRegex (skip blocking but continue enrichment; still applies after include)
6. Extract IP addresses from configured IP headers (ipHeaders) in the order they are defined
7. Apply IP header strategy (ipHeaderStrategy) to determine which IPs to process:
   - **CheckAll**: Process all found IP addresses for CIDR and private rules
   - **CheckFirst**: Process only the first IP address found
   - **CheckFirstNonePrivate**: Process first non-private IP, fallback to first private IP if no public IPs found
8. If `mode` is `enrich` or `enrichandblock`: look up each selected IP and write `countryHeader` plus `requestHeaderEnrich` (first public country wins)
9. If `mode` is `block` or `enrichandblock`: apply private and CIDR rules per selected IP, then country allow/block from `countryHeader` only
10. Apply default allow/deny if no country rule matches [defaultAllow]

**Important Notes:**
- Country allow/block uses the single `countryHeader` value (first public IP written). A later hop’s country is **not** checked. `CheckAll` still applies CIDR and private rules to every selected IP.
- If a public proxy appears later in `X-Forwarded-For` and you previously relied on `CheckAll` to country-block that hop, use `ipHeaderStrategy` (`CheckFirst` / `CheckFirstNonePrivate`) to choose which IP’s country is written, `allowedIPBlocks` / `blockedIPBlocks` for that hop’s address, or omit that hop from `ipHeaders`. There is no setting that country-checks every hop.
- With `CheckFirst` or `CheckFirstNonePrivate`: only the selected IP(s) are evaluated for CIDR and private rules.
- On lookup modes, `countryHeader` is initially set to `PRIVATE` and only overridden by the first real country found.
- Ignored HTTP verbs: Requests using verbs in `ignoreVerbs` skip all blocking logic but still receive GeoIP enrichment
- Included paths: When `includedPathsRegex` is set, only matching requests can be blocked. Other requests skip blocking but still receive GeoIP enrichment. The regex is a public URL match, not a secret — `bypassHeaders` is stronger for health checks
- Excluded paths: Requests matching `excludedPathsRegex` skip all blocking logic but still receive GeoIP enrichment. Exclude is evaluated after include and still wins. Same as include: a public URL is not a secret; `bypassHeaders` is stronger for health checks

---

📄 This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

🌍 This project includes IP2Location LITE data available from [`lite.ip2location.com`](https://lite.ip2location.com/database/ip-country).

🌍 This project includes [IPinfo Lite](https://ipinfo.io) data (CC-BY-SA 4.0). IP address data powered by [IPinfo](https://ipinfo.io).

🌍 This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com.