---
url: https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/blob/ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3/bouncer.go
title: crowdsec-bouncer-traefik-plugin bouncer.go
fetched: 2026-08-26
authority: source
ref: github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin@ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3:bouncer.go
---

Root file. Package name `crowdsec_bouncer_traefik_plugin` (hyphens from the module path replaced by underscores).

Imports helper subpackages of the same module:

```
github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/pkg/cache
github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/pkg/captcha
github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/pkg/configuration
github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/pkg/ip
github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/pkg/logger
```

Exports the Traefik entry points in this root package:

- `func CreateConfig() *configuration.Config`
- `func New(_ context.Context, next http.Handler, config *configuration.Config, name string) (http.Handler, error)`

`Config` itself lives in the `configuration` subpackage. `CreateConfig` still sits on the imported root package, which is what Traefik evals.
