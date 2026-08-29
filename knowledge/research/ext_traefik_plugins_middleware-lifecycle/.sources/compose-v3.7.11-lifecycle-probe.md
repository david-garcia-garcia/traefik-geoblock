---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/lifecyclereview/docker-compose.yml
title: compose traefik:v3.7.11 New ctx probe
fetched: 2026-08-28
authority: ticket
---

Host: `traefik:v3.7.11` local plugin bind-mount. Two whoami routes (`geoblock2@docker` on `/foo`, `geoblock@docker` on `/bar`). Temporary `LIFECYCLE-PROBE` logs in plugin `New` plus a goroutine on `ctx.Done()`. Reload by adding then removing an unrelated labeled whoami (no geoblock middleware). Plugin YAML unchanged.

At first `New` (12:45:11Z): `ctxType=*context.valueCtx`, `ctxErr=<nil>`, `hasDeadline=false`, `cause=<nil>`. One `New` per middleware.

After start of throwaway router (12:45:45Z): `Configuration received` then both prior probes `ctx.Done` (`ctxErr=context canceled`, `cause=context canceled`), then two new `New` with fresh probeIDs (~1 ms later).

After remove of throwaway (12:46:15Z): same Done-then-New pair.

Throwaway router rule failed to parse (`PathPrefix(/lifecycle-probe)` missing backticks). The docker config still applied; cancel still ran.
