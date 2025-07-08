# 🛡️ Traefik Geoblock Plugin

This plugin was forked from [nscuro/traefik-plugin-geoblock: traefik plugin to whitelist requests based on geolocation](https://github.com/nscuro/traefik-plugin-geoblock) and remains compatible with the original plugin.

[![Build Status](https://github.com/david-garcia-garcia/traefik-geoblock/actions/workflows/ci.yml/badge.svg)](https://github.com/david-garcia-garcia/traefik-geoblock/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/david-garcia-garcia/traefik-geoblock)](https://goreportcard.com/report/github.com/david-garcia-garcia/traefik-geoblock)
[![Latest GitHub release](https://img.shields.io/github/v/release/david-garcia-garcia/traefik-geoblock?sort=semver)](https://github.com/david-garcia-garcia/traefik-geoblock/releases/latest)
[![License](https://img.shields.io/badge/license-Apache%202.0-brightgreen.svg)](LICENSE)  

A Traefik plugin that allows or blocks requests based on IP geolocation using IP2Location database.

> 🌍 This project includes IP2Location LITE data available from [`lite.ip2location.com`](https://lite.ip2location.com/database/ip-country).

## ✨ Features

- Block or allow requests based on country of origin (using ISO 3166-1 alpha-2 country codes)
- Whitelist specific IP ranges (CIDR notation) - supports both inline configuration and directory-based files
- Blacklist specific IP ranges (CIDR notation) - supports both inline configuration and directory-based files
- Optional bypass using custom headers
- Configurable handling of private/internal networks
- Customizable error responses
- Flexible logging options
- Hot-swap database updates - automatic IP2Location database updates with zero downtime

## 📥 Installation

It is possible to install the [plugin locally](https://traefik.io/blog/using-private-plugins-in-traefik-proxy-2-5/) or to install it through [Traefik Plugins]([Plugins](https://plugins.traefik.io/plugins)).

### Local Plugin Installation

Create or modify your Traefik static configuration

```yaml
experimental:
  localPlugins:
    geoblock:
      moduleName: github.com/david-garcia-garcia/traefik-geoblock
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
```

## 🧪 Testing and development

You can spin up a fully working environment with docker compose:

```powershell
docker compose up --build
```

The codebase includes a full set of integration and unit tests:
```powershell
# Run unit tests
go test

# Run integration tests
.\Test-Integration.ps
```

## ⚙️ Configuration

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

When this environment variable is set, the plugin will automatically look for `IP2LOCATION-LITE-DB1.IPV6.BIN` and `geoblockban.html` files in the specified directory if they are not found in their configured locations.

### Example Docker Compose Setup

```yaml
version: "3.7"

services:
  traefik:
    image: traefik:latest
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
          databaseFilePath: "/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock/IP2LOCATION-LITE-DB1.IPV6.BIN"
          # Can be:
          # - Full path: /path/to/IP2LOCATION-LITE-DB1.IPV6.BIN
          # - Directory: /path/to/ (will search for IP2LOCATION-LITE-DB1.IPV6.BIN recursively). 
          # Use /plugins-storage/sources/ if you are installing from plugin repository.
          # - Empty: automatically searches using fallback locations (see below)
          # 
          # Fallback search order when file is not found:
          # 1. TRAEFIK_PLUGIN_GEOBLOCK_PATH environment variable directory
          
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
          # - "x-original-forwarded-for"  # Load balancer
          
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
          logLevel: "info"                  # Available: debug, info, warn, error
          logFormat: "json"                 # Available: json, text
          logPath: "/var/log/geoblock.log"  # Empty for Traefik's standard output
          logBannedRequests: true           # Log blocked requests. They will be logged at info level.
          fileLogBufferSizeBytes: 1024      # Buffer size for file logging in bytes (default: 1024)
          fileLogBufferTimeoutSeconds: 2    # Buffer timeout for file logging in seconds (default: 2)
          # File logging uses buffered writes for better performance. The buffer is flushed when:
          # - The buffer reaches fileLogBufferSizeBytes size
          # - fileLogBufferTimeoutSeconds seconds have passed since the last flush
          # - The logger is closed/shutdown

          #-------------------------------
          # Database Auto-Update Settings
          #-------------------------------
          databaseAutoUpdate: true                   
          # Enable automatic database updates with hot-swapping. Updates check every 24 hours
          # and immediately on startup if the current database is older than 1 month.
          # Updated databases are hot-swapped without requiring middleware restart.
          # Make sure you whitelist in your FW domains ["download.ip2location.com", "www.ip2location.com"]
          databaseAutoUpdateDir: "/data/ip2database" 
          # Directory to store updated databases. This must be a persistent volume in the traefik pod.
          # The plugin uses a singleton pattern - multiple middlewares with identical configurations
          # share the same database factory and hot-swap operations.
          databaseAutoUpdateToken: ""                # IP2Location download token (if using premium)
          databaseAutoUpdateCode: "DB1"              # Database product code to download (if using premium)

          #-------------------------------
          # Response header settings
          #-------------------------------  
          countryHeader: "X-IPCountry"  
          # Optional header to store the country code in
          # you can use this to add the header to the access logs
          # and see where all your trafik is coming from
          # make sure to include the header in the logs: accesslog.fields.headers.names.X-IPCountry=keep


```

### 🔄 Processing Order

The plugin processes requests in the following order:

1. Check if plugin is enabled
2. Check bypass headers
3. Extract IP addresses from configured IP headers (ipHeaders)
4. For each IP:
   - Check if it's in private network range [allowPrivate]
   - Check allowed/blocked IP blocks [allowedIPBlocks + allowedIPBlocksDir, blockedIPBlocks + blockedIPBlocksDir] (most specific match wins)
   - Look up country code 
   - Check allowed/blocked countries [allowedCountries, blockedCountries]
   - Apply default allow/deny if no rules match [defaultAllow]

If any IP in the chain is blocked, the request is denied.

### 📝 Log Format

When using JSON logging, the following fields are included in **blocked request** log entries (note: allowed requests are not logged):

- `time`: Timestamp of the request in ISO 8601 format
- `level`: Log level (debug, info, warn, error)
- `msg`: Log message describing the action
- `plugin`: Plugin identifier
- `ip`: The IP address that triggered the action
- `ip_chain`: Full chain of IP addresses from X-Forwarded-For header
- `country`: Country code or "PRIVATE" for internal networks
- `host`: Request host header
- `method`: HTTP method used
- `phase`: Processing phase where the action occurred:
  - `ip_allow_private`: Private network check
  - `ip_block`: IP block rules check
  - `country_block`: Country rules check
  - `default`: Default allow/deny rule
- `path`: Request path

Example log entry:
```json
{
    "time": "2025-03-01T19:24:04.414051815Z",
    "level": "INFO",
    "msg": "blocked request",
    "plugin": "geoblock@docker",
    "ip": "172.18.0.1",
    "ip_chain": "",
    "country": "PRIVATE",
    "host": "localhost:8000",
    "method": "GET",
    "phase": "ip_allow_private",
    "path": "/bar"
}
```



---

📄 This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
