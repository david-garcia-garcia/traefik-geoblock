---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/forwardedheaders/forwarded_header_test.go
title: pkg/middlewares/forwardedheaders/forwarded_header_test.go
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/middlewares/forwardedheaders/forwarded_header_test.go
---

`insecure true` with incoming XFF / URI / Method / TLS-cert / Prefix / underscore variants: all kept, including `X_forwarded_for`.

`insecure false`, no trustedIPs, incoming XFF and underscore variants: all those headers empty after ServeHTTP.

`insecure false`, trustedIPs `10.0.1.100`, RemoteAddr `10.0.1.100:80`: incoming XFF `10.0.1.0, 10.0.1.12` kept.

`insecure false`, same trustedIPs, RemoteAddr `10.0.1.101:80` (not in list): XFF and other incoming managed headers empty.

CIDR trusted `1.2.3.4/24` with peer `1.2.3.156:80` keeps XFF; peer `10.0.1.101:80` drops it.

RemoteAddr `10.0.1.101:80` and empty X-Real-Ip → `X-Real-Ip` becomes `10.0.1.101`.

`insecure true` with incoming `X-Real-Ip: 10.0.1.12` keeps `10.0.1.12` (does not overwrite with RemoteAddr).

Untrusted Connection-header abuse case: incoming `X-Real-Ip: foo` becomes empty when RemoteAddr is unset (delete then rewrite cannot SplitHostPort).
