---
url: local compose traefik:v3.7.11 + throwaway plugin yaegi-generic-probe
title: Yaegi v0.16.1 Table[T] load probe
fetched: 2026-08-28
authority: ticket
---

Image: `traefik:v3.7.11` (`Traefik version 3.7.11 built on 2026-08-19T06:38:56Z`). Yaegi from Traefik: `github.com/traefik/yaegi@v0.16.1`.

Throwaway module `github.com/david-garcia-garcia/yaegi-generic-probe` in `%TEMP%\yaegi-generic-probe` (not committed). Manifest `testData` required or load dies with `missing TestData` before Yaegi runs.

Local `NewTable[*db]()` + two `Open` on key `k`: plugins stay enabled; `New` logs `GENERIC-PROBE same=true id=7`.

Package-level `var dbs = reclaim.NewTable[*db]()`: `failed to import plugin code` / `constant definition loop`.

Package-level `var dbs *reclaim.Table[*db]` or helper `func table() *reclaim.Table[*db]`: process panic `invalid memory address or nil pointer dereference` in `yaegi/interp.nodeType2` (`interp/type.go:1083`), `gta`, `importSrc`, `newInterpreter` (`middlewareyaegi.go:159`).

`var dbs any` then `dbs.(*reclaim.Table[*db])` in `New`: `type not found: reclaim.Table[*yaegi_generic_probe.db]`.
