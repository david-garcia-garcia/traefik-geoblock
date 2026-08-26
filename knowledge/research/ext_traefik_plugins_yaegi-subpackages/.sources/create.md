---
url: https://plugins.traefik.io/create
title: Developing Traefik Plugins
fetched: 2026-08-26
authority: official
---

Two plugin kinds: Yaegi (Go) and Wasm.

A Traefik Yaegi plugin is developed in Go. A Traefik plugin is essentially a Go package. Yaegi plugins are executed on the fly by Yaegi, a Go interpreter embedded in Traefik. They are not pre-compiled.

The page points at the official skeleton repos: Middleware Demo Plugin and Provider Demo Plugin. It does not list required exported symbols on this page; it defers architecture to those repos.

Catalog packaging:

- Sources hosted in a public GitHub repository.
- Versioned with a git tag (catalog fetches via a Go module proxy).
- Root must have `.traefik.yml` with valid `testData`.
- Root must have a valid `go.mod`.
- If you have package dependencies, they must be vendored and added to the GitHub repository.

This page does not mention plugin-owned subpackages (`pkg/…`). It does not say helpers must live in the root package.
