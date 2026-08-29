# Plugin packages

## Language

**Plugin root**:
The module-root Go package Traefik Yaegi evals. It MUST export `New` and `CreateConfig`. It owns process-table instance reclaim for one Plugin per middleware name and config.
_Avoid_: putting ServeHTTP or geo policy in this package; putting instance reclaim in `pkg/geoblock`.

**Subpackage**:
A Go package under `pkg/` that the plugin root imports. Yaegi can load these; crowdsec-bouncer uses the same layout.

## Overview

Entrypoints stay at `github.com/david-garcia-garcia/traefik-geoblock`. Helpers live in `pkg/<name>`. Root `New` reuses one Plugin; `pkg/geoblock` constructs and serves it.

## How to use

- Root package exports `Config`, `CreateConfig`, and `New`. Those are the Yaegi entrypoints. Root `New` hashes the prepared config, `reclaim.Open`s the incarnation, and attaches this router’s `next` plus this generation’s provider.
- Put Plugin, ServeHTTP, Prepare, OpenDatabase, and NewCore in `pkg/geoblock`. That package constructs one Plugin; it does not call `reclaim.Open`.
- Do not put `New` / `CreateConfig` only in a subpackage — Traefik evals the module root.

## Key files

- `.traefik.yml` `import:` — module root
- `plugin.go` — Yaegi `New` / `CreateConfig` / `Config` alias and instance reclaim
- `pkg/geoblock` — Plugin, ServeHTTP, Prepare, OpenDatabase, NewCore
- `pkg/` — helpers (`dbprovider`, `dbwrappers`, `dbsource`, `reclaim`, `ip2location`, `ipinfo`, `maxmind`, `dbutils`, `fileutils`, `iplookup`, `logging`)
