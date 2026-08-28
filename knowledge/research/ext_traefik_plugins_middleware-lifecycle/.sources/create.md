---
url: https://plugins.traefik.io/create
title: Developing Traefik Plugins
fetched: 2026-08-28
authority: official
---

Two plugin kinds: Yaegi (Go) and Wasm.

A Traefik Yaegi plugin is a Go package executed on the fly by Yaegi. Architecture is deferred to the Middleware Demo Plugin and Provider Demo Plugin repos.

This page does not list `Close` or `Stop`. It does not describe per-router instantiation or dynamic-config reload of `New`.
