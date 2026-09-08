---
url: https://raw.githubusercontent.com/david-garcia-garcia/traefik-modsecurity/main/pkg/reclaim/table_test.go
title: traefik-modsecurity pkg/reclaim/table_test.go
fetched: 2026-09-08
authority: source
ref: david-garcia-garcia/traefik-modsecurity@645f4a25d5023fc42e615e02240cab29799b39c5:pkg/reclaim/table_test.go
---

27 Test* functions; same names as local pkg/reclaim/table_test.go.
TestTable_HashChangeProof: after waitKeyMsg(MsgDispose, "A"), upstream adds:
  // Close runs on the life goroutine after cancel; the dispose log can win the race.
  waitUntil(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(ended)==1 && ended[0]=="A" })
Local omits waitUntil and reads ended immediately — race with async Close.
Stress coverage present upstream: ReclaimRacesFire (40 rounds), ZeroGraceOpenRacesCancel (40 rounds), stale fire after reclaim/reset, concurrent Open/cancel.
