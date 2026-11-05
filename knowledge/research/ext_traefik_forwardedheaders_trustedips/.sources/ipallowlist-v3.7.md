---
url: https://doc.traefik.io/traefik/v3.7/reference/routing-configuration/http/middlewares/ipallowlist/
title: Traefik HTTP Middlewares IPAllowList | Traefik | v3.7
fetched: 2026-11-03
authority: official
---

`ipAllowList` accepts/refuses based on client IP. `sourceRange` is required.

`ipStrategy.depth`: position in `X-Forwarded-For` starting from the right. 0 means no depth. If greater than the number of IPs, client IP is empty. If higher than 0, `excludedIPs` is not evaluated.

`ipStrategy.excludedIPs`: scan `X-Forwarded-For` and select the first IP not in the list. Ignored if `depth` is specified.

If no strategy is set, match `sourceRange` against the Remote address found in the request.

Passlisting happens before proxying. The previous network hop is appended to `X-Forwarded-For` only during the last stages of proxying, after passlisting. During passlisting that hop is not yet in `X-Forwarded-For`.

Depth example: XFF `"10.0.0.1,11.0.0.1,12.0.0.1,13.0.0.1"`, depth 1 → `13.0.0.1`; depth 3 → `11.0.0.1`; depth 5 → empty. "Real" client at depth 4 would be `10.0.0.1` but depth 2 uses `12.0.0.1`.

excludedIPs examples (same XFF `10,11,12,13`): exclude `12,13` → `11.0.0.1`; exclude `15,13` → `12.0.0.1`; exclude `10,13` → `12.0.0.1`; exclude none of them (`15,16`) → `13.0.0.1`; exclude all present → empty.
