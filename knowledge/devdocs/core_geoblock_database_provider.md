# Database provider

## Language

**DatabaseProvider**:
The interface the plugin uses to open a geo database, look up a country, and run auto-update.
_Avoid_: calling an IP2Location SDK type from `plugin.go`.

**databaseProvider**:
Traefik Config key that names the implementation. Empty defaults to `ip2location`. Unknown values fail `New`.

## Overview

`New` calls `openDatabaseProvider`, which switches on `databaseProvider` and stores a `dbprovider.Provider`. Request path calls `LookupCountry`. Each vendor owns its Traefik keys (`ip2location_*` today).

## How to use

- Set `databaseProvider` (or leave empty for `ip2location`).
- Set that vendor's prefixed fields. Pass them only into that vendor's constructor.
- Unprefixed IP2Location keys are deprecated aliases: `applyDeprecatedIP2LocationSettings` copies them when the `ip2location_` field is unset, then defaults the code to `DB1`. Do not default the new code in `CreateConfig` or Traefik old-only `databaseAutoUpdateCode` is ignored.
- Call `LookupCountry(ip)` from the plugin. Map errors through existing `banIfError`.
- Add a later vendor by implementing `Provider` and adding a switch branch. Do not type-assert a concrete wrapper in `plugin.go`.

## Pattern snippet

```go
db, err := openDatabaseProvider(cfg, bootstrapLogger)
country, err := p.db.LookupCountry(ip)
```

## Key files

- `pkg/dbprovider` — interface
- `pkg/ip2location` — only implementation (`DatabaseConfig`)
- `plugin.go` — `openDatabaseProvider`
