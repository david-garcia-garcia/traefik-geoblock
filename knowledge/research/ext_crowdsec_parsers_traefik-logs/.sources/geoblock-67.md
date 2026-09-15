---
url: https://github.com/PascalMinder/geoblock/issues/67
title: Traefik Logs Parser Fails on Non-JSON Logs
fetched: 2026-09-08
authority: ticket
---

Reporter mixed Traefik access + GeoBlock stdout. GeoBlock lines: `INFO: GeoBlock: 2024/12/26 11:36:01 allow local IPs: true` (plain text).

Error: `failed to run filter : invalid character 'I' looking for beginning of value` on `UnmarshalJSON(evt.Parsed.message, evt.Unmarshaled, "traefik") in ["", nil]`.

Environment: CrowdSec v1.6.4, GeoBlock v0.2.8, Traefik v3.

Expected: parser ignores non-JSON plugin lines. Owner asked reporter for compatible log format; reporter cited CrowdSec traefik-logs expecting CLF or Traefik access JSON per Traefik docs.
