---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/README.md
title: traefik-middleware-utilities README
fetched: 2026-09-14
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b:README.md
---

Shared Go libraries for Traefik middlewares that must run under Yaegi. Reclaim is how a middleware keeps a client or gate across a Traefik reload.

Package reclaim/: keep one Go value per key for the life of a Traefik plugin instance.

Call table.Open from the plugin constructor (New), on a *Table the plugin package holds (reclaim.New). Pass Traefik's constructor context, not req.Context(). First Open for a key runs create once. Later Opens return that value. Traefik reload cancels the old context; the table sleeps the value; a new New that opens the same key before grace ends wakes it.

Pass Sleep / Wake / Close as reclaim.Hooks. Prefix keys when more than one type shares a table.

Pattern: var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace}); table.Open(..., reclaim.Hooks{Sleep, Wake, Close}).

Pester suite -Suite reclaim loads fake local plugins under Traefik v3.7.11 so reclaim runs under Yaegi.
