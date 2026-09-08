# Issues

- [ ] note large  `knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md`
  Why: Yaegi strips the method set from a value returned through `func() (any, error)`, so the optional lifecycle never runs interpreted and this change's saving is real only for compiled callers.
- [ ] note large  `knowledge/debt/2026-09-08-release-wrapper-db-handle-on-sleep.md`
  Why: a sleeping wrapper still holds its database open, and releasing it needs a `Wake` failure policy the human has to decide.
- [ ] note large  `knowledge/debt/2026-09-08-port-reclaim-lifecycle-to-modsecurity.md`
  Why: `pkg/reclaim` is a bidirectionally synced copy and this change is its first real behavioural divergence from `traefik-modsecurity`.
- [ ] note  `knowledge/debt/2026-09-08-cancelable-database-download.md`
  Why: `HTTPGet` is not cancelable, so a `Sleep` that lands during a download blocks an `Open` for as long as the transfer takes.
