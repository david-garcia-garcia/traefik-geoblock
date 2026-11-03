---
url: https://doc.traefik.io/traefik/contributing/security-decisions/
title: Traefik Security Decisions
fetched: 2026-11-03
authority: official
---

Section "Forwarded Headers and Client Identity":

The trust boundary for forwarded headers is entrypoint-level. `forwardedHeaders.trustedIPs` and `forwardedHeaders.insecure` decide once, at the entrypoint, before any middleware runs, whether a client's forwarding headers are trusted.

Traefik deliberately does not re-decide per middleware. A middleware passing through a header the entrypoint accepted is behaving as designed.

In scope as a vulnerability: a path that reintroduces or reconstructs a trusted value after entrypoint sanitisation; a middleware that makes a pre-authentication decision on a header the operator declared untrusted.

Typical reports name `X-Forwarded-*` or `Forwarded`.
