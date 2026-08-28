---
url: https://doc.traefik.io/traefik/master/extend/extend-traefik/
title: Extend Traefik
fetched: 2026-08-28
authority: official
---

Traefik supports Yaegi and WASM plugin systems. Yaegi plugins are a Go package executed by an embedded interpreter. Adding a plugin requires changing install (static) configuration.

This page does not define middleware `New` teardown, `Close`, or reload behavior.
