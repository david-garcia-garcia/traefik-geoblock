# Issues

- [ ] note large  `knowledge/debt/2026-09-08-release-wrapper-db-handle-on-sleep.md`
  Why: a sleeping wrapper still holds its database open, and releasing it needs a `Wake` failure policy the human has to decide.
- [ ] note large  `knowledge/debt/2026-09-08-port-reclaim-lifecycle-to-modsecurity.md`
  Why: `pkg/reclaim` is a bidirectionally synced copy and this change is its first real behavioural divergence from `traefik-modsecurity`.
