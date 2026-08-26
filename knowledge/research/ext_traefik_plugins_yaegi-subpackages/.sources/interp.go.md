---
url: https://github.com/traefik/yaegi/blob/fcb76d1ece0c3edc2548c39aa5b170475d2261bb/interp/interp.go
title: interp.Options
fetched: 2026-08-26
authority: source
ref: github.com/traefik/yaegi@fcb76d1ece0c3edc2548c39aa5b170475d2261bb:interp/interp.go
---

`Options.GoPath` comment: “GoPath sets GOPATH for the interpreter.”

`interp.New` assigns `i.opt.context.GOPATH = options.GoPath`.

Yaegi therefore resolves interpreted package imports against that GOPATH (classic `$GOPATH/src/<import path>`), not against Go module mode.
