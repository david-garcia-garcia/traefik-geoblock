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

- **No per-request GeoIP API** — lookups use a local IP2Location BIN or IPinfo MMDB
- **Minimal memory footprint** — no application-level cache; the database format is read in place
- **Offline after load** — no outbound call unless you enable auto-update
- **Hot-swappable database updates** — new files load without restarting Traefik

This architecture ensures consistent response times and eliminates external service bottlenecks, making it ideal for high-traffic environments and air-gapped deployments.

**Expected throughput** (`go test -bench=BenchmarkPlugin -benchmem` on a local Intel Core Ultra 7 265K). `Lookup` always reads the full geo row (`Get_all`). `ServeHTTP` reuse is the Traefik path (request/recorder reused). Country-only vs full `requestHeaderEnrich` on the same BIN is the same lookup; extra headers are cheap.

| Database | `Lookup` | `ServeHTTP` reuse | Typical cost |
| --- | --- | --- | --- |
| LITE DB1 (country) | ~148k ops/s | ~144k ops/s | ~6.7–7.0 µs, 4–5 allocs, ~540 B |
| LITE DB1 + ASN LITE | ~48k ops/s | ~52k ops/s | ~19–21 µs, 9–10 allocs, ~1200 B |
| Paid DB8 (country only) | ~37k ops/s | ~46k ops/s | ~22–27 µs, 13–14 allocs, ~1840 B |
| Paid DB8 + full enrich (country/region/city/isp/domain) | ~46k ops/s | ~45k ops/s | ~22 µs, 20 allocs, 1944 B |
| Paid DB8 + ASN LITE (full enrich) | ~27k ops/s | ~22k ops/s | ~38–45 µs, 18–26 allocs, ~2500 B |

## Geo enrichment and observability

Every request that reaches the plugin gets a local GeoIP lookup. The result is written onto the **request** (`requestHeaderEnrich`), not the HTTP response:

- **Applications** behind Traefik see the same headers (`X-Geo-Country`, city, ASN, …) and can branch on them without calling a GeoIP service.
- **Traefik access logs** can `keep` those names so every line is geo-aware (dashboards, SIEM, compliance).
- Headers are **not** copied onto the response, so browsers and other clients do not see them.

Blocking is optional. You can run the middleware with `defaultAllow: true` and empty country lists and still enrich.

> **Recommended**: map the fields you need with `requestHeaderEnrich`, and set `logStatusDetailHeader` if you also want the allow/block reason (`pass:{reason}` / `block:{reason}`). Keep those header names in Traefik access logs.

### Available Headers

| Config Setting | Purpose | Example Values |
|----------------|---------|----------------|
| `requestHeaderEnrich` | Geo metadata on the request. Every mapped header is written. Missing values are `null`. Country on a private IP is `PRIVATE`. | `X-Geo-Country: US`, `X-Geo-Region: null` |
| `countryHeader` | **Deprecated.** Use `requestHeaderEnrich` with key `country` | `US`, `DE`, `PRIVATE` |
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

The plugin looks up IPs with one of three providers (`databaseProvider`):

| Provider | Config value | Bundled seed (when available) |
| --- | --- | --- |
| IP2Location (default) | `ip2location` or empty | `IP2LOCATION-LITE-DB1.IPV6.BIN` (country). ASN is a separate BIN and is **not** bundled. |
| IPinfo | `ipinfo` | `ipinfo_{code}.mmdb` — `lite` (bundled seed, country + ASN), `core`, or `plus` (region/city filled) |
| MaxMind | `maxmind` | `GeoIP2-Country-Test.mmdb` (official **dummy** Country fixture, not a live GeoLite file). Operator-supplied GeoLite2/GeoIP2 Country or City MMDBs use nested `country.iso_code`. |

Only the selected provider’s files are opened. Unused vendor paths are ignored.

**Finding the bundled files.** Traefik does not put the plugin tree on the process working directory. Set `TRAEFIK_PLUGIN_GEOBLOCK_PATH` to the directory that contains the plugin (the local clone, or the registry unpack such as `/plugins-storage/sources/github.com/david-garcia-garcia/traefik-geoblock`). The plugin searches that directory for the bundled filenames when no other file is configured. Without this variable, empty seed paths fail unless the file is already on an auto-update volume.

**Recommendation: configure auto-update** and point the auto-update directory at a persistent volume. The bundled snapshots (and any static seed file you copy in) go stale. Auto-update downloads a current database, stores a dated copy, and hot-swaps it without restarting Traefik.

How auto-update works:

- Turn it on with `ip2location_databaseAutoUpdate` / `ip2location_asnDatabaseAutoUpdate` / `ipinfo_databaseAutoUpdate` / `maxmind_databaseAutoUpdate`, and set the matching `*AutoUpdateDir` (required; must survive container restarts).
- On startup the plugin uses the newest dated file already in that directory. A 24-hour ticker then checks again.
- IP2Location LITE DB1 downloads from the public CDN with no token. ASN LITE and paid IP2Location packages need `ip2location_databaseAutoUpdateToken` and the official `file=` package code. IPinfo Lite downloads only when `ipinfo_databaseAutoUpdateToken` is set; without a token an error is logged and the seed stays in use. MaxMind downloads only when `maxmind_databaseAutoUpdateToken` is `accountId:licenseKey`; without a valid token an error is logged and the seed stays in use. Default MaxMind edition is `GeoLite2-Country`.
- New files are stored as `YYYYMMDD_…` in the auto-update directory. The prefix is the date inside the file (IP2Location BIN header, IPinfo MMDB `build_epoch`). IP2Location downloads when the open BIN is older than 30 days. IPinfo skips the download when that dated MMDB is less than 24 hours old.
- Same config shares one factory (one ticker). See [Network Requirements](#network-requirements) for the download hosts.

`ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, `ipinfo_databaseFilePath`, and `maxmind_databaseFilePath` are **seeds / fallbacks**, not the live copy once auto-update has stored a file. Resolution order for each database the provider needs:

1. **Auto-update directory** — newest dated file already there (when auto-update is on).
2. **Configured seed path** — the matching `*_databaseFilePath` (file, or a directory searched for the default filename).
3. **Bundled database** — the committed seed, when that provider ships one, found via `TRAEFIK_PLUGIN_GEOBLOCK_PATH`.

If none of those exist, plugin creation fails (except an empty IP2Location ASN path, which means “no ASN”).

## Network Requirements

**For automatic database updates to function, ensure your firewall allows outbound HTTPS connections to:**

- `download.ip2location.com` — IP2Location LITE DB1 (no token)
- `www.ip2location.com` — IP2Location token downloads (paid packages and ASN LITE)
- `ipinfo.io` — IPinfo Lite MMDB (token required; the response redirects to IPinfo’s download CDN)
- `download.maxmind.com` — MaxMind GeoLite2/GeoIP2 permalink (`accountId:licenseKey`; the response redirects to MaxMind’s download host)

> **Note:** If automatic updates are disabled (`ip2location_databaseAutoUpdate`, `ip2location_asnDatabaseAutoUpdate`, `ipinfo_databaseAutoUpdate`, and `maxmind_databaseAutoUpdate` all false), no external network access is required and the plugin operates entirely offline. IP2Location LITE DB1 can auto-update without a token. ASN LITE, IPinfo Lite, and MaxMind downloads are not attempted unless their download token is set.

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

# Run integration tests
.\Test-Integration.ps1
```

Token-protected downloads (IP2Location paid/LITE-with-token, ASN LITE, IPinfo Lite) are optional and local-only. Copy `.env.example` to `.env`, set the tokens, then run `.\Test-Integration.ps1`. Compose starts profile `local-tokens` (`/tokendb`) when `IP2LOCATION_DOWNLOAD_TOKEN` is set. Pester waits for Traefik log lines (`database updated successfully`, `hot-swapped`, `IPinfo database updated`). Cases skip when the matching token is empty. CI does not load `.env`.

```

## Configuration

### Environment Variables

The plugin supports the following environment variable for configuration:

- **`TRAEFIK_PLUGIN_GEOBLOCK_PATH`**: Directory path used as fallback location for database and HTML files when they are not found in the specified paths or when paths are empty/omitted.

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

When this environment variable is set, the plugin will automatically look for `IP2LOCATION-LITE-DB1.IPV6.BIN`, `ipinfo_lite.mmdb`, `GeoIP2-Country-Test.mmdb`, and `geoblockban.html` in the specified directory if they are not found in their configured locations. An ASN BIN is opened when `ip2location_asnDatabaseFilePath` points at one, or after ASN auto-update has downloaded one. ASN auto-update downloads only when `ip2location_databaseAutoUpdateToken` is set. IPinfo auto-update downloads only when `ipinfo_databaseAutoUpdateToken` is set. MaxMind auto-update downloads only when `maxmind_databaseAutoUpdateToken` is `accountId:licenseKey`.

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
      - ./IP2LOCATION-LITE-DB1.IPV6.BIN:/plugins-storage/IP2LOCATION-LITE-DB1.IPV6.BIN
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
          enabled: true                   # Enable/disable the plugin entirely
          defaultAllow: false             # Default behavior when no rules match (false = block)
          
          #-------------------------------
          # Database Configuration
          #-------------------------------
          databaseProvider: ip2location   # ip2location (default), ipinfo, or maxmind. Empty defaults to ip2location.
          # Vendor keys are prefixed: ip2location_*, ipinfo_*, or maxmind_*. See those sections below.
          
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
          # - Full path: /path/to/geoblockban.html
          # - Directory: /path/to/ (will search for geoblockban.html recursively). Use /plugins-storage/sources/ if you are installing from plugin repository.
          # - Empty: returns only status code
          # 
          # Fallback search order when file is not found:
          # 1. TRAEFIK_PLUGIN_GEOBLOCK_PATH environment variable directory
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
          # IP2Location Database
          #-------------------------------
          ip2location_databaseFilePath: "/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock/IP2LOCATION-LITE-DB1.IPV6.BIN"
          # Seed geo BIN (country; region/city/isp/domain on DB8+). Used only when
          # auto-update is off, or the auto-update dir has no BIN yet.
          # Deprecated aliases still work if the ip2location_ keys are unset:
          # databaseFilePath, databaseAutoUpdate, databaseAutoUpdateDir,
          # databaseAutoUpdateToken, databaseAutoUpdateCode.
          # Prefer the ip2location_ keys; a startup warning is logged when aliases are set.
          # Can be:
          # - Full path: /path/to/IP2LOCATION-LITE-DB1.IPV6.BIN
          # - Directory: /path/to/ (will search for IP2LOCATION-LITE-DB1.IPV6.BIN recursively).
          # Use /plugins-storage/sources/ if you are installing from plugin repository.
          # - Empty: TRAEFIK_PLUGIN_GEOBLOCK_PATH, or skip if auto-update dir already has a BIN.
          ip2location_databaseAutoUpdate: true                    
          # Enable automatic database updates with hot-swapping. Updates check every 24 hours
          # and immediately on startup if the current database is older than 1 month.
          # Updated databases are hot-swapped without requiring middleware restart.
          # Make sure you whitelist in your FW domains ["download.ip2location.com", "www.ip2location.com"]
          ip2location_databaseAutoUpdateDir: "/data/ip2database" 
          # Directory to store updated databases. This must be a persistent volume in the traefik pod.
          # The plugin uses a singleton pattern - multiple middlewares with identical configurations
          # share the same database factory and hot-swap operations.
          ip2location_databaseAutoUpdateToken: ""
          # Download token from https://lite.ip2location.com (or a paid account).
          # LITE DB1 auto-update works without a token (public CDN).
          # ASN LITE and paid packages require this token. file= is the package code.
          ip2location_databaseAutoUpdateCode: "DB8BINIPV6"       # Official IP2Location package code for file= (e.g. DB8BINIPV6, DB1LITEBINIPV6). Sent unchanged when a token is set. Not a short product like DB8.

          ip2location_asnDatabaseFilePath: ""
          # Seed ASN BIN (https://lite.ip2location.com/database-asn). Same rule as
          # ip2location_databaseFilePath: used only when the auto-update dir has no ASN BIN yet.
          # Leave empty if you do not need ASN or you rely on token auto-update. The geo BIN has no ASN.
          ip2location_asnDatabaseAutoUpdate: false
          # Opt-in. Downloads and hot-swaps the ASN BIN only when
          # ip2location_databaseAutoUpdateToken is set. The public lite CDN does
          # not host IP2LOCATION-LITE-ASN.IPV6.BIN (404). Register at
          # lite.ip2location.com and use file=DBASNLITEBINIPV6 (~264MB).
          # Without a token the flag is ignored and an error is logged.
          # Reuses ip2location_databaseAutoUpdateDir.
          ip2location_asnDatabaseAutoUpdateCode: "DBASNLITEBINIPV6"
          # Official file= package code. Use DBASNLITEBIN for the IPv4-only ASN BIN.

          #-------------------------------
          # IPinfo Database
          #-------------------------------
          # Official package code (same idea as ip2location_databaseAutoUpdateCode):
          # lite (default, free, CC-BY-SA 4.0), core, plus.
          # Seed and dated files are ipinfo_{code}.mmdb
          # (https://ipinfo.io/data/ipinfo_{code}.mmdb?token=$TOKEN).
          # The repo ships ipinfo_lite.mmdb. Empty path uses that file when code is lite.
          # Core/Plus fill region and city; Lite leaves those empty.
          ipinfo_databaseFilePath: ""
          ipinfo_databaseAutoUpdate: false
          ipinfo_databaseAutoUpdateDir: "/data/ipinfo"
          ipinfo_databaseAutoUpdateToken: ""
          # Account token. Required to download. Lite cap: 10 downloads/day/IP.
          # Without a token an error is logged and the seed file is used.
          ipinfo_databaseAutoUpdateCode: "lite"

          #-------------------------------
          # MaxMind / GeoLite2 Database
          #-------------------------------
          # Official edition IDs: GeoLite2-Country (default), GeoLite2-City,
          # GeoIP2-Country, GeoIP2-City. ASN-only editions are rejected.
          # Country allow/block uses nested country.iso_code (not IPinfo country_code).
          # The repo ships GeoIP2-Country-Test.mmdb (MaxMind official dummy fixture).
          # Empty path uses that file. It is not a live GeoLite database.
          # This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com
          maxmind_databaseFilePath: ""
          maxmind_databaseAutoUpdate: false
          maxmind_databaseAutoUpdateDir: "/data/maxmind"
          maxmind_databaseAutoUpdateToken: ""
          # accountId:licenseKey (HTTP Basic Auth). Required to download.
          # Without a valid token an error is logged and the seed file is used.
          maxmind_databaseAutoUpdateCode: "GeoLite2-Country"

          #-------------------------------
          # Request header settings
          #-------------------------------  
          countryHeader: "X-IPCountry"
          # Deprecated. Prefer requestHeaderEnrich (below) with metadata key country.
          # If set, the plugin copies this header name onto requestHeaderEnrich as country
          # unless that header is already mapped. An explicit enrich mapping wins.

          requestHeaderEnrich:
            X-Geo-Country: country
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
          # asn needs the ASN LITE BIN (ip2location_asnDatabaseFilePath, or
          # asnDatabaseAutoUpdate plus a download token).
          # IPinfo Lite fills country, country_name, continent, continent_code,
          # isp (as_name), domain (as_domain), and asn (AS15169 form).
          # region and city stay empty.
          
          logStatusDetailHeader: "X-Geoblock-Decision"
          # Optional header to add the decision to the REQUEST
          # Format: "pass:{reason}" or "block:{reason}"
          # See the Observability section for all possible values
          # Example access log config: accesslog.fields.headers.names.X-Geoblock-Decision=keep


```

### Processing Order

The plugin processes requests in the following order:

1. Check if plugin is enabled
2. Check bypass headers
3. Check if HTTP verb is in ignoreVerbs list (skip blocking but continue enrichment)
4. If includedPathsRegex is set and the request does not match, skip blocking but continue enrichment
5. Check if request matches excludedPathsRegex (skip blocking but continue enrichment; still applies after include)
6. Extract IP addresses from configured IP headers (ipHeaders) in the order they are defined
7. Apply IP header strategy (ipHeaderStrategy) to determine which IPs to process:
   - **CheckAll**: Process all found IP addresses (original behavior)
   - **CheckFirst**: Process only the first IP address found
   - **CheckFirstNonePrivate**: Process first non-private IP, fallback to first private IP if no public IPs found
8. For each selected IP:
   - Check if it's in private network range [allowPrivate]
   - Check allowed/blocked IP blocks [allowedIPBlocks + allowedIPBlocksDir, blockedIPBlocks + blockedIPBlocksDir] (most specific match wins)
   - Look up country code 
   - Check allowed/blocked countries [allowedCountries, blockedCountries]
   - Apply default allow/deny if no rules match [defaultAllow]

**Important Notes:**
- With `CheckAll` strategy: If any IP in the chain is blocked, the request is denied
- With `CheckFirst` or `CheckFirstNonePrivate` strategies: Only the selected IP(s) are evaluated; the request is denied only if the selected IP is blocked
- Country header behavior: Header is initially set to "PRIVATE" and only overridden by the first real country found, preventing private IPs from overriding legitimate geolocation information
- Ignored HTTP verbs: Requests using verbs in `ignoreVerbs` skip all blocking logic but still receive GeoIP enrichment
- Included paths: When `includedPathsRegex` is set, only matching requests can be blocked. Other requests skip blocking but still receive GeoIP enrichment. The regex is a public URL match, not a secret — `bypassHeaders` is stronger for health checks
- Excluded paths: Requests matching `excludedPathsRegex` skip all blocking logic but still receive GeoIP enrichment. Exclude is evaluated after include and still wins. Same as include: a public URL is not a secret; `bypassHeaders` is stronger for health checks

---

📄 This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

🌍 This project includes IP2Location LITE data available from [`lite.ip2location.com`](https://lite.ip2location.com/database/ip-country).

🌍 This project includes [IPinfo Lite](https://ipinfo.io) data (CC-BY-SA 4.0). IP address data powered by [IPinfo](https://ipinfo.io).