# Traefik logs parser line shapes

What `crowdsecurity/traefik-logs` accepts, what triggers `UnmarshalJSON` errors, and implications for GeoBlock stdout mixed into Traefik container logs. Not how this product configures CrowdSec.

Pinned hub implementation: [crowdsecurity/hub@ce8e034](https://github.com/crowdsecurity/hub/tree/ce8e034852d062cbe3f768d7afd89ecf12f07be7) (`parsers/s01-parse/crowdsecurity/traefik-logs.yaml`). Hub page version: **v1.5**.

## Two input branches: Traefik extended CLF or JSON

The parser runs only when `evt.Parsed.program startsWith 'traefik'` (acquisition labels the stream). It then tries two sibling nodes on `evt.Parsed.message`:

1. **CLF** — grok on Traefik’s extended Common Log Format (nginx-style prefix plus Traefik router/server/duration suffix). Matches hub test fixture lines like `172.17.0.1 - - [08/Dec/2021:09:16:05 +0000] "GET /path HTTP/1.1" 200 414 "-" "-" 500 "test@docker" "http://172.17.0.3:80" 0ms`.
2. **JSON** — filter `TrimSpace(evt.Parsed.message) startsWith "{" && UnmarshalJSON(evt.Parsed.message, evt.Unmarshaled, "traefik") in ["", nil]`, then statics map **Traefik access-log JSON field names** into `evt.Parsed` / `evt.Meta`.

Official hub copy: parser supports access logs in CLF “defined here for Traefik” and JSON ([traefik-logs hub page](https://hub.crowdsec.net/author/crowdsecurity/configurations/traefik-logs), [traefik collection](https://hub.crowdsec.net/author/crowdsecurity/collections/traefik)). Traefik documents three access formats (`common`, `genericCLF`, `json`); the CLF grok targets Traefik extended CLF, not bare generic CLF without the trailing Traefik fields ([Traefik access logs](https://doc.traefik.io/traefik/reference/install-configuration/observability/logs-and-accesslogs/)).

Extracts: [.sources/traefik-logs.yaml.md](.sources/traefik-logs.yaml.md), [.sources/traefik-logs-hub.md](.sources/traefik-logs-hub.md), [.sources/traefik-access-log-format.md](.sources/traefik-access-log-format.md)

## Q1 — Any JSON vs Traefik access-log field names

**UnmarshalJSON does not validate Traefik field names.** CrowdSec’s helper unmarshals the blob with generic `json.Unmarshal` into `interface{}` and stores it under `evt.Unmarshaled.<key>` (here `traefik`). Any syntactically valid JSON object passes the filter ([crowdsec@master:pkg/exprhelpers/jsonextract.go](https://github.com/crowdsecurity/crowdsec/blob/master/pkg/exprhelpers/jsonextract.go)).

**Useful access-log parsing requires Traefik access-log keys.** After unmarshal, the parser reads specific names documented by Traefik (`ClientHost`, `ClientAddr`, `RequestMethod`, `RequestPath`, `DownstreamStatus`, `Duration`, `RouterName`, `request_User-Agent`, etc.) in statics. Missing keys leave parsed/meta fields empty or nil; they are not rejected at unmarshal time.

The hub JSON test lines include Traefik access fields plus extra keys (`level`, `msg`, `time`, `entryPointName`, …) — the parser expects that shape, not arbitrary slog schemas ([hub `.tests/traefik_json/traefik_json.log`](https://github.com/crowdsecurity/hub/blob/master/.tests/traefik_json/traefik_json.log)).

Extracts: [.sources/jsonextract.go.md](.sources/jsonextract.go.md), [.sources/traefik_json.log.md](.sources/traefik_json.log.md)

## Q2 — CLF vs JSON only

**Both.** CLF node runs first (grok). JSON node runs when the line trim-starts with `{` and unmarshals. Hub and collection explicitly state CLF and JSON support. Only Traefik **access** log shapes are in scope — not Traefik process `log` output unless it happens to match CLF or is JSON that the statics can map.

## Q3 — Valid JSON that is not a Traefik access line

Example: slog JSON `{"time":"…","level":"INFO","msg":"…","plugin":"geoblock"}`.

- **UnmarshalJSON:** succeeds (valid JSON); no `invalid character` error.
- **Filter:** passes if line starts with `{`.
- **Statics:** Traefik field expressions (`evt.Unmarshaled.traefik.RequestMethod`, `int(evt.Unmarshaled.traefik.Duration)`, …) resolve to nil/empty; some `int(...)` on nil may emit `failed to run RunTimeValue` warnings (same class of issue noted for other JSON parsers — [hub#986](https://github.com/crowdsecurity/hub/issues/986)).
- **Downstream:** root statics still set `meta.log_type` to `http_access-log` whenever this parser node succeeds, so non-access JSON can be mis-tagged as HTTP access unless scenarios filter on populated fields.

**Contrast — Traefik process log JSON** (`log.format: json`): lines start with `{` and unmarshal successfully but lack access fields; same mis-tagging risk.

**Contrast — plain text** (`INFO: GeoBlock: …`): does not start with `{`; JSON branch skipped entirely (see Q4).

Extracts: [.sources/traefik-logs.yaml.md](.sources/traefik-logs.yaml.md), [.sources/hub-986.md](.sources/hub-986.md)

## Q4 — Non-JSON lines and current hub behavior

**Current hub (`traefik-logs` v1.5):** JSON branch guard is `TrimSpace(evt.Parsed.message) startsWith "{" && UnmarshalJSON(...)`. For plain-text lines (GeoBlock `INFO: GeoBlock: …`, Traefik common-format debug `time=… level=debug msg=…`), the guard is false; with expr `&&` short-circuit, **UnmarshalJSON is not invoked** and the `invalid character 'I' looking for beginning of value` error should not occur on those lines.

If neither CLF grok nor JSON branch matches, the line is not parsed as Traefik access; hub tests cover only CLF and JSON access fixtures — no assertion that alien lines error.

**Reported GeoBlock / mixed-stdout pain** ([geoblock#67](https://github.com/PascalMinder/geoblock/issues/67)): reporter saw `UnmarshalJSON` failure on `INFO: GeoBlock` with CrowdSec v1.6.4; error snippet shows only the `UnmarshalJSON(...)` clause (no visible `startsWith "{"` guard), consistent with an **older hub parser** before the guard or a truncated filter in the log message. Reporter expectation: CrowdSec should ignore non-JSON plugin lines.

**CrowdSec maintainer stance on mixed streams** ([crowdsec#3279](https://github.com/crowdsecurity/crowdsec/issues/3279)): non-access lines written to an acquisition the parser treats as Traefik access logs can produce warnings; preferred fix is **separate access-log output** from process/plugin stdout, not parser silence for all non-access shapes.

**Remaining gap even with current hub:** JSON-shaped **non-access** lines (Traefik `log.format: json`, plugin slog JSON) still enter the JSON branch and unmarshal cleanly — they are not “ignored” like plain text.

Extracts: [.sources/geoblock-67.md](.sources/geoblock-67.md), [.sources/crowdsec-3279.md](.sources/crowdsec-3279.md), [.sources/traefik-logs.yaml.md](.sources/traefik-logs.yaml.md)

## GeoBlock stdout guidance (inference from above)

| Line shape | UnmarshalJSON error? | Parsed as access? |
|------------|----------------------|-------------------|
| Traefik extended CLF access line | No | Yes (if grok matches) |
| Traefik access JSON | No | Yes (fields mapped) |
| Plain text `INFO: …` / common log | No (current hub) | No |
| JSON slog / process JSON starting with `{` | No | Partial / mis-tagged |

Plain text plugin logs are safe against `UnmarshalJSON` on **current** hub. JSON plugin logs or mixed JSON process logs are not automatically ignored.

## Sources

- https://hub.crowdsec.net/author/crowdsecurity/configurations/traefik-logs
- https://hub.crowdsec.net/author/crowdsecurity/collections/traefik
- https://github.com/crowdsecurity/hub/blob/ce8e034852d062cbe3f768d7afd89ecf12f07be7/parsers/s01-parse/crowdsecurity/traefik-logs.yaml
- https://doc.traefik.io/traefik/reference/install-configuration/observability/logs-and-accesslogs/
- https://github.com/crowdsecurity/crowdsec/blob/master/pkg/exprhelpers/jsonextract.go
- https://github.com/PascalMinder/geoblock/issues/67
- https://github.com/crowdsecurity/crowdsec/issues/3279
- https://github.com/crowdsecurity/hub/issues/986
