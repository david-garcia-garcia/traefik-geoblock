---
url: https://doc.traefik.io/traefik/reference/install-configuration/observability/logs-and-accesslogs/
title: Traefik Logs and AccessLogs reference
fetched: 2026-09-08
authority: official
---

**Process logs (`log`):** separate from access logs; default format `common` or optional `json`; written to stdout unless `filePath` set.

**Access logs (`accessLog`):** default format Traefik extended CLF (`common`); also `genericCLF` or `json`. Default output stdout.

**Traefik CLF fields:** remote IP, user, timestamp, request line, downstream status, content-length, referrer, user-agent, request count since start, router name, server URL, duration ms.

**JSON access fields (subset cited by parser):** `ClientHost`, `ClientAddr`, `RequestMethod`, `RequestPath`, `RequestProtocol`, `DownstreamStatus`, `DownstreamContentSize`, `Duration`, `RouterName`, `RequestHost`, `RequestAddr`, `ServiceAddr`, `StartUTC`, `StartLocal`, plus optional header keys like `request_User-Agent`.
