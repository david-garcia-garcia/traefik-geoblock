## 1. Token file= pass-through

- [x] 1.1 When `DatabaseAutoUpdateToken` is set, set `file=` to `DatabaseAutoUpdateCode` unchanged (no `IP2LOCATION-LITE-…ZIP` wrap, no `BINIPV6` suffix).
- [x] 1.2 Leave the empty-token `liteDownloadURL` path unchanged.

## 2. Tests and docs

- [x] 2.1 Add unit tests that lock `file=` for `DB8BINIPV6`, `DB1LITEBINIPV6`, and `DB8` (exact), and that reject the old ZIP filename for `DB1` + token.
- [x] 2.2 Update README: with a token, `databaseAutoUpdateCode` is the official package code (example `DB8BINIPV6`), not a short product (`DB8`).
