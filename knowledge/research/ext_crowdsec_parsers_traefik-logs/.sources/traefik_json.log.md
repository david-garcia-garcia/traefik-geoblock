---
url: https://github.com/crowdsecurity/hub/blob/master/.tests/traefik_json/traefik_json.log
title: Hub traefik JSON parser test fixture
fetched: 2026-09-08
authority: source
ref: github.com/crowdsecurity/hub@ce8e034852d062cbe3f768d7afd89ecf12f07be7:.tests/traefik_json/traefik_json.log
---

Sample line fields: `ClientAddr`, `ClientHost`, `DownstreamStatus`, `Duration`, `RequestMethod`, `RequestPath`, `RouterName`, `StartUTC`, `time`, plus extras `level`, `msg`, `entryPointName`, `request_User-Agent`, etc.

Confirms parser test corpus is Traefik access JSON, not arbitrary slog objects.
