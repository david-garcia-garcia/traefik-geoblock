---
url: https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/blob/ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3/.traefik.yml
title: crowdsec-bouncer-traefik-plugin .traefik.yml
fetched: 2026-08-26
authority: source
ref: github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin@ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3:.traefik.yml
---

```
displayName: Crowdsec Bouncer Traefik Plugin
type: middleware
import: github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin
```

`import` is the module root, not a `pkg/…` subpath. No `basePkg` field (Traefik will derive `crowdsec_bouncer_traefik_plugin` from the last path segment).
