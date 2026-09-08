# Explore
IssueKey: 2026-09-08-non-json-logs

## Concepts

Upstream [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) reports CrowdSec `crowdsecurity/traefik-logs` failing `UnmarshalJSON` on GeoBlock lines shaped `INFO: GeoBlock: …`. The reporter asked CrowdSec to skip non-JSON; this repo can only change what we emit.

```
Traefik stdout (one stream)
  ├─ access log (JSON or CLF) ──► CrowdSec traefik-logs
  └─ plugin slog ───────────────► same parser
```

Current hub YAML (`crowdsecurity/hub` `parsers/s01-parse/crowdsecurity/traefik-logs.yaml`) gates JSON with `TrimSpace(evt.Parsed.message) startsWith "{"` before `UnmarshalJSON`. Text that does not start with `{` is not unmarshaled. Issue-era errors show `UnmarshalJSON` with no that guard (CrowdSec 1.6.4, 2024-12).

Measured on this tree (throwaway `go run`, 2026-09-08, worktree HEAD):

| Logger | Format | First byte | `json.Unmarshal` | `INFO: GeoBlock:` prefix |
|--------|--------|------------|------------------|--------------------------|
| `CreateConfig` default | `text` | — | — | — |
| `NewBootstrap` | always text | `t` (`time=`) | false | false |
| `NewOwner` | always text | `t` | false | false |
| `New` + CreateConfig format | text | `t` | false | false |
| `New` + `json` | json | `{` | true | false |

`TestNew_JSONFormat` only checks substrings. No test locks the #67 prefix absence, the CrowdSec `startsWith "{"` predicate, or full-line JSON decode. `CreateConfig` tests do not assert `LogFormat`.

`pkg/logging` has no usage packet. Existing tests already call `New` / `NewBootstrap`; implement can extend `pkg/logging/logging_test.go` and `pkg/geoblock` CreateConfig without a new Language term.

No client-address / Host identity work. No `New` reclaim / `pkg/reclaim` change.

## Decisions

- The filed prefix (`INFO: GeoBlock:`) is **not** in this product. The remaining CrowdSec risk on DestBranch is default **text** slog (`time=…`) plus missing proof. This run adds tests that prove the prefix is gone and that `logFormat: json` lines are CrowdSec-safe JSON objects. No default flip. No bootstrap/owner format parameter.
- Fold the invariant onto `core_geoblock_observability_decision-header` (stdout already owned there) rather than a new leaf, unless FindSpecHost says otherwise.
- Do not change CrowdSec Hub. Do not emit Traefik access-log field names on plugin lines.

## Open questions

- Q: Does current CrowdSec hub still UnmarshalJSON every Traefik line, or only lines that start with `{`?
  Decision: assumed — hub master YAML uses `TrimSpace(evt.Parsed.message) startsWith "{"` before UnmarshalJSON; this run treats current hub as skip-non-JSON. Tests still lock our bytes so #67’s prefix cannot return and json format is valid JSON.
  By: explore

- Q: Should CreateConfig default `logFormat` become `json`?
  Decision: assumed — no. The filed format is absent; flipping the default is extra. Operators who want JSON already have `logFormat`.
  By: explore

- Q: Must `NewBootstrap` / `NewOwner` honor `logFormat` so a json config never emits text?
  Decision: assumed — not this run. Current hub skips text. Note as follow-up.
  By: explore

- Q: Does CrowdSec require Traefik access-log JSON fields on plugin lines, or is any JSON object enough to avoid the unmarshal warning?
  Decision: assumed — any JSON object avoids UnmarshalJSON failure; access-log fields are for access events only. slog JSON (`time`, `level`, `msg`, `plugin`) is enough. We do not emit access-log keys.
  By: explore
