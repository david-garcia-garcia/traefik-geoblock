## Context

See proposal.md for motivation. Token download is built in `downloadAndUpdateDatabase` (`autoupdate.go`). Today `file=` is `IP2LOCATION-LITE-{DatabaseAutoUpdateCode}.IPV6.BIN.ZIP`. Official download is `https://www.ip2location.com/download?token={TOKEN}&file={DATABASE_CODE}`. The operator supplies that DATABASE_CODE.

## Goals / Non-Goals

**Goals:**
- Token path: `file=` = configured `DatabaseAutoUpdateCode` with no rewrite.
- Unit tests lock `file=` without a network call or a real token.
- README says the field is the official package code (example `DB8BINIPV6`).

**Non-Goals:**
- Mapping `DB8` → `DB8BINIPV6` or any other suffix.
- Custom HTTP client or redirect handling unless a measured download fails.
- Committing or documenting a live download token.
- Changing `tools/dbdownload` or the bundled LITE DB1 file.

## Decisions

- **Pass through.** Human: use exactly what the operator provides. Alternative considered: derive `{code}BINIPV6` from short products — rejected this run.
- **Keep on-disk glob on the same configured string.** Files become `*IP2LOCATION-LITE-DB8BINIPV6.IPV6.BIN` when that is what they set.
- **Tests assert the URL string**, not a live GET.
- **Extract first `.BIN`** stays as today.

## Risks / Trade-offs

- [Default `DB1` + token sends `file=DB1` (CSV, not IPv6 BIN)] → Mitigation: README. Operators who own paid DB8 IPv6 BIN must set `DB8BINIPV6`.
- [HTML error body with HTTP 200] → Mitigation: existing status check; no HTML sniffing unless measured.
- [R2 redirect after May 2025] → Mitigation: Go `http.Get` follows redirects; revisit only after a failed download.

## Migration Plan

- Deploy. Set `databaseAutoUpdateToken` and `databaseAutoUpdateCode` to the official package code from the IP2Location download page (this run: `DB8BINIPV6`).
- Rollback: previous builds still wrap `file=` as a ZIP name; free LITE path is unchanged.
