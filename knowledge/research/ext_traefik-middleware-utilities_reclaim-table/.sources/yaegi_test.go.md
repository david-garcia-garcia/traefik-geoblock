---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/reclaim/yaegi_test.go
title: reclaim/yaegi_test.go at v1.0.1
fetched: 2026-09-14
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b:reclaim/yaegi_test.go
---

Interpreted probes against Yaegi (github.com/traefik/yaegi/interp + stdlib symbols). GOPATH copy of non-test reclaim sources.

TestYaegi_CreateAnyTypeSwitchDoesNotMatch: Yaegi v0.16.1 synthesizes a create func() (any, error) return with no methods, so a type-switch to a Sleep interface does not match (got "no").

TestYaegi_OpenHooksRunSleepWakeClose: Open Hooks Sleep/Wake/Close run under the interpreter (each count ≥ 1). Skips when race detector on (Yaegi v0.16.1 select races inside the interp on context cancel).

TestYaegi_GraceExpireDoesNotHang: interpreted concurrent last-holder drop with positive grace (1ms) must dispose; fails in 3s if AfterFunc is replaced by a missed interp._select. Comment: DestBranch hang before AfterFunc, Go 1.21.13 missed Close.

Hookprobe pattern matches production: reclaim.New(Config{Grace}), Open(..., reclaim.Hooks{Sleep, Wake, Close, EnforceCloseBeforeOpen: true}).
