## Context

See proposal.md — Why. `countryHeader` is already the write/read bridge. Enrich writes it; block reads it and uses `banIfError` when it is missing. The gap is Traefik-visible proof and the README title.

## Goals / Non-Goals

**Goals:**
- H1 names Geoblock and Geoenrich.
- One compose router lists enrich then block.
- One compose router is block only (`banIfError` true, no lookup).
- Pester hits both PathPrefixes.

**Non-Goals:**
- Renaming the module, repo, or plugin catalog id.
- Changing `mode` or `banIfError` defaults.
- New package tests for the chain.

## Decisions

- **Title:** `# 🌍 Traefik Geoblock and Geoenrich`. Geoblock stays first so the catalog name still matches. Alternative: Geoenrich first — rejected; operators search Geoblock.
- **Chain labels:** `geoblock-chain-enrich` then `geoblock-chain-block` on `/enrichthenblock`. Traefik applies middlewares in list order.
- **Block-only:** `/blockonly`, `geoblock-blockonly`, `banIfError=true`. Pester sends a public `X-Real-IP` and no country header.
- **Fold** onto `core_geoblock_plugin_request-mode`. No new spec family.

## Risks / Trade-offs

- [Wrong middleware order] → list enrich first. Block-first would 403 every public IP with default `banIfError`.
- [Private IP on block-only] → send `8.8.8.8`, not localhost, so `allowPrivate` does not hide the missing header.

## Migration Plan

Docs and tests only. No operator config change. Rollback is revert.
