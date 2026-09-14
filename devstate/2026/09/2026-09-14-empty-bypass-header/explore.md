# Explore

Measured on dest code at `2026-09-14-empty-bypass-header` (throwaway `pkg/geoblock` test, then deleted). `newRoute` accepted `BypassHeaders: {"X-Bypass": ""}`. A US client (`X-Real-IP: 8.8.8.8`, `blockedCountries: [US]`, `defaultAllow: false`) with no `X-Bypass` on the request reached next (`418` teapot) with `X-Geoblock-Decision: pass:bypass_header`. Failure reproduced. No product fix in this phase.

No active OpenSpec change (`openspec list --json` → `changes: []`). Live specs do not name `bypassHeaders` (searched `openspec/` for `bypassHeader` / `bypass_header`). Propose will pick a host; this phase does not.

Devdocs consumed: `knowledge/devdocs/index.md` → `index_core_geoblock.md` → request-mode, plugin instance, test harness, plugin packages. No Language gap to write: bypass is already `BypassHeaders` / `PhaseBypassHeader`. Request-mode usage already shows `skipBlock` before the block stage. No packet produced.

Research consumed: `knowledge/research/index.md` → `index_ext_traefik.md` (Yaegi subpackages, middleware lifecycle, generics). No YAML/mapstructure decode finding. No third-party write: the contract this change owns is the Go `BypassHeaders` map after Traefik decode, not Traefik’s YAML parser.

This change does not reconstruct client address, user, tenant, Host, or trust hop. `bypassHeaders` is an operator skip token. Header bytes on the request are owned by `net/http.Header`; `blockSkipReason` must keep reading that owner (`Get` / `Values`), not re-derive presence from a different signal.

## Concepts

```
Prepare(cfg)                    ServeHTTP (mode blocks)
    │                                │
    │  today: no BypassHeaders check │  enrich may still run
    ▼                                ▼
 NewCore copies map ──► blockSkipReason
                            ignoreVerbs
                            include/exclude regex
                            for each bypass pair:
                              Get(header) == expected
                              Get absent → ""
                              "" == "" → PhaseBypassHeader
                            ▼
                     setDecisionLogHeader pass:bypass_header
                     next (dest tests: teapot 418, not 200)
```

- `pkg/geoblock/config.go` `BypassHeaders` — header name → expected value; match skips the block stage. Comment has no non-empty invariant. `CreateConfig` starts an empty map.
- `Prepare` — constructor gate. Validates mode, `DisallowedStatusCode`, non-empty `IPHeaders` (`len == 0` reject), `IPHeaderStrategy`, `CountryHeader` (TrimSpace empty → default), catalog. `ModeDisabled` returns before those checks. Does not read `BypassHeaders`.
- `plugin.go` `New` and `newTestPlugin` both call `Prepare` then `NewCore`. `NewCore` assigns `cfg.BypassHeaders` with no copy or value check.
- `blockSkipReason` (`pkg/geoblock/plugin.go`) — only production caller is `ServeHTTP`. Order in code: ignore verbs, include regex, exclude regex, then bypass. README processing order lists `bypassHeaders` before `ignoreVerbs` (pre-existing trail mismatch; not this ticket).
- `req.Header.Get(name)` — first value, or `""` if the header is absent. That is why an empty configured value matches every omitted header.
- `req.Header.Values(name)` — non-empty slice iff the header is present (including a present empty value). Desired present-header check uses this so a leftover empty map entry cannot match an absent header.
- Decision header: `pass:bypass_header` (`LogStatusPass` + `PhaseBypassHeader`) when `logStatusDetailHeader` is set. Dest policy tests use `noopHandler` teapot (`418`) as pass-through; the claimed `200` is “reached the backend,” not the package-test status.
- Existing tests: `plugin_policy_test.go` `BypassHeaders` uses non-empty secrets; omitted header with a non-empty value is `403`. `plugin_observe_test.go` matching non-empty `X-Bypass` is `pass:bypass_header`. `plugin_config_test.go` `EmptyIPHeaders` is the Prepare-reject sibling. No empty-bypass case on dest (`zzz_proof_*` absent).
- `IPHeaders` README already says the list cannot be empty. `bypassHeaders` example is non-empty (`X-Internal-Request: "true"`) and does not forbid empty values.

## Decisions

- Proceed with both layers named in Desired: `Prepare` rejects empty bypass values (loud-fail, same constructor gate as empty `IPHeaders`), and `blockSkipReason` requires `req.Header.Values(header)` non-empty before comparing. Reject-only still matches absent headers if an empty entry reaches the plugin map (`NewCore` without `Prepare`). Ignore-only (skip empty entries at request time, still load) hides the misconfiguration. Documented on the open-question row.
- Empty means `strings.TrimSpace(value) == ""` in `Prepare` (whitespace-only counts as empty).
- Do not change match behavior for non-empty bypass values. Present-header before compare does not change those cases: absent + non-empty expected already fails `Get` equality; present match still matches.
- Product tests in `pkg/geoblock/plugin_config_test.go` (Prepare reject, sibling of `EmptyIPHeaders`) and `pkg/geoblock/plugin_policy_test.go` (empty value must not skip blocking / must not write `pass:bypass_header` for a blocked-country request). No `zzz_proof_*` filenames. Pass-through assertions use dest’s teapot next handler, not literal `200`.
- README: one operator-facing line that empty `bypassHeaders` values are rejected, next to the existing example, matching the `ipHeaders` empty-reject sentence.
- `ModeDisabled` keeps today’s early return: do not validate `BypassHeaders` when mode is disabled (same as `IPHeaders`).
- Empty header *names* stay out of scope.
- YAML/env producers are not a separate contract: validate the Go map; present-header covers a leftover empty entry. No `knowledge/research/` write this phase.
- Smallest durable delta: `Prepare` + `blockSkipReason` + those tests + README line. Do not copy the map, do not retarget other skip reasons.

## Open questions

- Q: Reject empty `bypassHeaders` values in `Prepare`, ignore them at request time, or both?
  Rank: bounded asked — existing `Prepare` / `blockSkipReason` contracts; 6 `Prepare` call sites (searched `**/*.go` for `Prepare(` and `geoblock.Prepare(`): `plugin.go` `New`, `newTestPlugin`, four in `plugin_mode_test.go`) and 1 `blockSkipReason` caller (`ServeHTTP` in `pkg/geoblock/plugin.go`; searched `**/*.go`); Desired names loud-fail in `Prepare` shaped like `IPHeaders` and also require the header present via `Values`
  Decision: assumed — both. `Prepare` rejects empty values so a Traefik/`New` config cannot load. `blockSkipReason` requires `Values` non-empty before compare so a leftover empty map entry cannot match an absent header. Ignore-only would still apply a broken config. Reject-only would still open the gate if `NewCore` ran without `Prepare`. Do not also skip empty `expectedValue` at request time; `Prepare` owns empty config.
  By: explore

- Q: Are whitespace-only bypass values empty?
  Rank: additive incidental — new check inside the `Prepare` validation this change adds; Unknowns asks whether whitespace-only should count as empty, no In-scope line names `TrimSpace`
  Decision: assumed — yes; `strings.TrimSpace(value) == ""` is empty and fails `Prepare`. Ticket names a trimmed secret as a producer of `""`. `CountryHeader` already uses `TrimSpace` for empty. Spaces-only would not match `Get` on an absent header, but rejecting it fails the same leftover-secret class at load.
  By: explore

- Q: Is an empty bypass header *name* with a non-empty value in scope?
  Rank: additive incidental — extra `Prepare` check this change could add in the same loop; Unknowns says the ticket is empty *value*
  Decision: assumed — no. Do not reject empty names. Bound the ask to empty values.
  By: explore

- Q: Does Traefik/Yaegi YAML `X-Bypass:` (nothing after the colon) materialize as `""` in the map, or omit the key?
  Rank: additive asked — Unknowns names YAML decode; Desired is that an empty value cannot silently disable blocking, which is the Go map after decode
  Decision: assumed — do not depend on YAML omit vs `""`. Validate whatever lands in `BypassHeaders`. Present-header covers a leftover empty entry. `index_ext_traefik.md` has no YAML-decode finding; no research folder this phase.
  By: explore

- Q: Should `Prepare` reject empty bypass values when `mode` is `disabled`?
  Rank: additive incidental — extra work on the `ModeDisabled` early return; no criterion names it
  Decision: assumed — no. Keep the existing early return; `IPHeaders` is also skipped when disabled. A later mode change goes through `New`/`Prepare` again.
  By: explore

- Q: Document the operator-facing empty-value reject in `README.md`?
  Rank: additive asked — Affected lists `README.md` bypassHeaders contract if operator-facing reject is documented; `ipHeaders` already documents cannot-be-empty
  Decision: assumed — yes. One sentence next to the `bypassHeaders` example that empty values are rejected at plugin creation, matching the `ipHeaders` empty-reject line. Do not rewrite processing order in this change (README lists bypass before ignore verbs; code is the reverse).
  By: explore
