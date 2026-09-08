---
url: https://raw.githubusercontent.com/david-garcia-garcia/traefik-modsecurity/main/pkg/reclaim/table.go
title: traefik-modsecurity pkg/reclaim/table.go
fetched: 2026-09-08
authority: source
ref: david-garcia-garcia/traefik-modsecurity@645f4a25d5023fc42e615e02240cab29799b39c5:pkg/reclaim/table.go
---

Table/slot grace-lease state machine unchanged vs local.
stopValue parameter: upstream `value any`; local `v any` — same Close-on-closer behavior.
Open path: upstream uses `created`/`stored` locals; local uses `v` — same create-outside-lock, lost-create discard, bind, watch, fire semantics.
DefaultGrace 10s; MsgPut/Bind/Orphan/Reclaim/Dispose constants match.
