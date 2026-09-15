---
url: https://github.com/crowdsecurity/hub/blob/ce8e034852d062cbe3f768d7afd89ecf12f07be7/parsers/s01-parse/crowdsecurity/traefik-logs.yaml
title: crowdsecurity/traefik-logs parser YAML
fetched: 2026-09-08
authority: source
ref: github.com/crowdsecurity/hub@ce8e034852d062cbe3f768d7afd89ecf12f07be7:parsers/s01-parse/crowdsecurity/traefik-logs.yaml
---

Top filter: `evt.Parsed.program startsWith 'traefik'`. `onsuccess: next_stage`.

**CLF node:** grok `NGINXACCESS2` + Traefik suffix (`number_of_requests_received_since_traefik_started`, `traefik_router_name`, `traefik_server_url`, `request_duration_in_ms`) on `message`.

**JSON node filter:** `TrimSpace(evt.Parsed.message) startsWith "{" && UnmarshalJSON(evt.Parsed.message, evt.Unmarshaled, "traefik") in ["", nil]`.

JSON statics map Traefik access fields: `ClientHost`, `ClientAddr`, `RequestAddr`, `ServiceAddr`, `request_User-Agent`, `DownstreamContentSize`, `Duration`, `RouterName`, `time`, `RequestMethod`, `RequestPath`, `RequestProtocol`, `DownstreamStatus`, `RequestHost`.

Root statics always set `meta.log_type` = `http_access-log`, `meta.service` = `http`, plus http meta from `evt.Parsed.*`.
