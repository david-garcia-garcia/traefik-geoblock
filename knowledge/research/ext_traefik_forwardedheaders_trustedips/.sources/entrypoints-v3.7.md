---
url: https://doc.traefik.io/traefik/v3.7/reference/install-configuration/entrypoints/
title: Traefik EntryPoints Documentation | Traefik | v3.7
fetched: 2026-11-03
authority: official
---

`forwardedHeaders.insecure` (default false): always trust forwarded headers information (`X-Forwarded-*`). Recommended only for tests, not production.

`forwardedHeaders.trustedIPs`: IPs or CIDR from where Traefik trusts the forwarded headers information (`X-Forwarded-*`).

`forwardedHeaders.notAppendXForwardedFor` (default false): when true, Traefik will not append the client's `RemoteAddr` to `X-Forwarded-For`. Existing header is preserved as-is. If no `X-Forwarded-For` exists, none will be added.

`forwardedHeaders.connection`: Connection headers allowed to pass through the middleware chain before being removed. Removal of other Connection-listed fields happens as soon as Traefik handles the request, so those headers are not available when the request passes through the middleware chain.

Forwarded Headers section: "You can configure Traefik to trust the forwarded headers information (`X-Forwarded-*`)." Examples configure `trustedIPs` or `insecure: true` on the entrypoint.

ProxyProtocol (separate from forwardedHeaders): "The replacement of the remote client address will occur only for IP addresses listed in `trustedIPs`." `proxyProtocol.insecure` trusts every incoming connection and then `trustedIPs` has no effect.
