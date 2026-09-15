## Why

The committed IP2Location LITE BIN under `seeds/` last landed with #60 (2026-08-28). Official LITE is a newer file (11,702,411 bytes vs 10,349,586). Operators who do not run auto-update keep that older country map.

## What Changes

- Replace `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN` with the official free LITE DB1 IPv6 BIN from `https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP`.
- Leave `seeds/ipinfo_lite.mmdb` unchanged (official full Lite is token-gated; this run has no token).
- Leave `seeds/GeoIP2-Country-Test.mmdb` unchanged (byte-identical to `maxmind/MaxMind-DB` `test-data`).
- Do not change `tools/dbdownload` verify (`1.1.1.1` → `US`). Official LITE returns `AU` for that IP; the sentinel is a follow-up.

## Capabilities

### New Capabilities

### Modified Capabilities

No spec-level behavior change. Reserved catalog rows, Resolve, and open rules stay as specified. This change is a same-name byte replace of one bundled file. `skip_specs: true` is set on this change.

## Impact

- `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN`
- Package and integration tests that look up `8.8.8.8` / `1.1.1.1` against the LITE seed (`8.8.8.8` stays `US`; `1.1.1.1` stays `AU`)
- No public Config keys, no new vendor, no seed filename change
