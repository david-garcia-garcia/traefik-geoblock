---
url: https://github.com/crowdsecurity/crowdsec/issues/3279
title: Bug reading traefik.log
fetched: 2026-09-08
authority: ticket
---

User pointed CrowdSec at traefik.log; errors like `UnmarshalJSON : invalid character '-' after top-level value` on Traefik debug lines (common log format, not access JSON).

Maintainer (@LaurenceJJones): expected — lines are not access logs; CrowdSec only parses access logs. If process and access logs share one file, warnings continue; use separate access log file when possible.
