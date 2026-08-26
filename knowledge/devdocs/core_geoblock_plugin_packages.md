# Plugin packages

## Language

**Plugin root**:
The module-root Go package Traefik Yaegi evals. It MUST export `New` and `CreateConfig`.
_Avoid_: putting helper types in this package when they have their own job.

**Subpackage**:
A Go package under `pkg/` that the plugin root imports. Yaegi can load these; crowdsec-bouncer uses the same layout.

## Overview

Entrypoints stay at `github.com/david-garcia-garcia/traefik-geoblock`. Helpers live in `pkg/<name>`.

## How to use

- Add middleware behavior on the root package only.
- Put a second job in `pkg/<name>` and import it.
- Do not put `New` / `CreateConfig` in a subpackage.

## Key files

- `.traefik.yml` `import:` — module root
- `plugin.go` — entrypoints
- `pkg/` — helpers (`dbprovider`, `ip2location`, `dbutils`, `fileutils`, `iplookup`, `logging`)
