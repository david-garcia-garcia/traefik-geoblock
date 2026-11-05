---
url: https://doc.traefik.io/traefik/v3.5/reference/install-configuration/entrypoints/
title: Traefik EntryPoints Documentation | Traefik | v3.5
fetched: 2026-11-03
authority: official
---

v3.5 field table includes `forwardedHeaders.trustedIPs` and `forwardedHeaders.insecure` with the same wording as v3.7: trust `X-Forwarded-*` from listed IPs/CIDR, or always trust in insecure mode (tests only).

v3.5 field table does **not** list `forwardedHeaders.notAppendXForwardedFor` or `forwardedHeaders.connection`.

ProxyProtocol section is the same idea: remote client address replacement only for `trustedIPs`.
