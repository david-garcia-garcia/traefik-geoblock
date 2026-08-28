---
url: local compose traefik:v3.7.11 + %TEMP%\yaegi-generic-probe
title: Yaegi v0.16.1 store/assert workaround matrix
fetched: 2026-08-28
authority: ticket
---

Same image as the Table[T] probe. Cases in `cases/*.go`; runner copied each over `plugin.go` and recreated `yaegi-generic-probe-traefik`.

PASS (log `GENERIC-PROBE same=true`): 01 typed map in root; 02 `any` in root; 03 `box.Store.Any` + `(*db)` in root; 04 interface assert `*db`; 05 interface method; 06 `unsafe.Pointer`; 07 same-package `Table[T]` package var; 08 non-generic table; 09 local `hasID` assert; 12 `box.Put`/`Get` on `map[string]any`.

FAIL: 10 `var dbs *reclaim.Table[*db]` → process panic `nodeType2`. 11 `var dbs = reclaim.NewTable[*db]()` → `constant definition loop`. 13 type alias `reclaim.Table[*db]` → panic. 14 embed `*reclaim.Table[*db]` → panic.

03 and 12 confirmed on a second recreate (fresh container logs, 13:29:19Z and 13:30:18Z).
