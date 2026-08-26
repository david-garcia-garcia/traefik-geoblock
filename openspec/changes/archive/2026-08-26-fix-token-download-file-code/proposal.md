## Why

Token auto-update sends `file=IP2LOCATION-LITE-{code}.IPV6.BIN.ZIP`. The IP2Location download API expects the official package code the operator owns (`DB8BINIPV6`, `DB1LITEBINIPV6`, …) and returns 404 on the ZIP filename. Token download cannot succeed until `file=` is exactly that configured code.

## What Changes

- When `databaseAutoUpdateToken` is set, `file=` is `databaseAutoUpdateCode` unchanged. No product-to-package mapping.
- The plugin MUST NOT wrap the code as `IP2LOCATION-LITE-{code}.IPV6.BIN.ZIP`.
- Empty-token free LITE URL is unchanged.
- On-disk names still use the configured code string (`*IP2LOCATION-LITE-{databaseAutoUpdateCode}.IPV6.BIN`).
- README states that with a token, `databaseAutoUpdateCode` is the official IP2Location download package code (example: `DB8BINIPV6`). The download token is never stored in the repo.

## Capabilities

### New Capabilities

- `core_geoblock_database_token-download-file`: Token download `file=` is the configured package code, exactly.

### Modified Capabilities

- None. No baseline specs exist.

## Impact

- `autoupdate.go` token URL construction; tests that lock `file=`.
- `README.md` auto-update token/code docs (operators must set `DB8BINIPV6`, not `DB8`).
- `openspec/specs/domains.md` allowlist: first `core` / `geoblock` pair.
- Default `databaseAutoUpdateCode` remains `DB1`. With a token that sends `file=DB1` (CSV package), which is not an IPv6 BIN. Token users must set the official BIN code they own.
