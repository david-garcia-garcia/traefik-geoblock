## Context

See proposal.md — Why. `writeDefaultEnrichHeaders` seeds every `country` enrich header with
`PRIVATE` so a private-only chain still has a country for `allowPrivate`.
`writePublicLookupHeaders` deliberately does not overwrite that default on an empty record.
Before `mode`, that was enrich-only cosmetics. Since `mode`, `blockFromHeader` reads the same
header and `decide` maps `PRIVATE` to the `allowPrivate` verdict, so the cosmetic default
became a policy decision for addresses it was never meant to describe.

`Combined` fills only empty fields, and `binColumn` returns `country_short` raw while every
other column maps IP2Location's `-` to `""` through `usableMeta`. A catalog with an enabled
`bin` row therefore answers `-` for an unknown address — a non-empty country that already
routes to `defaultAllow`. The reserved `default_ip2location` row is enabled unless the
operator disables it, so this only bites catalogs with no enabled `bin` row.

## Goals / Non-Goals

**Goals:**
- An unresolved public hop reaches the country maps and `defaultAllow`.
- `PRIVATE` still means a private or loopback hop.
- The first hop that actually resolves still wins the country header.

**Non-Goals:**
- Changing `banIfError`. A miss is not a failed lookup.
- Changing `mode: block`, which must keep honouring an inbound `PRIVATE` from an upstream
  enrich hop.
- The `CheckAll` pass/deny ordering (see Risks).
- Enrichment completeness when the merged record has no country: the other mapped keys keep
  their `null` defaults, unchanged by this fix. Worth revisiting once an `mmdb`-only catalog
  is safe to run, but it is pre-existing and not part of this defect.
- `recordForLookup` writing the client IP into the country header on a lookup *error*.
  Pre-existing and untouched.
- Routing `country_short` through `usableMeta` for consistency with the other columns. That
  is a separate change and must not land before this one: on its own it turns the latent
  fail-open into a live one for every IP2Location user.

## Decisions

- **A distinct value, not `null`.** `blockFromHeader` treats a missing, empty, or `null`
  country as the `banIfError` case, so `null` would turn these into `block:error` wherever
  `banIfError` is on — including under `defaultAllow: true`, where they pass today. `XX`
  reaches `block:default_allow`, which is what README documents for an unknown country. It also keeps "lookup failed" (an operational
  fault) distinguishable from "no record" (a data gap) in `logStatusDetailHeader`.
- **`XX`, from ISO 3166-1 user-assigned codes** (`AA`, `QM`–`QZ`, `XA`–`XZ`, `ZZ`), which ISO
  will never allocate. `countryHeader` is consumed by things that expect a two-letter code, so
  a word like `UNKNOWN` would not fit. `ZZ` was considered and rejected: it is CLDR's own
  "Unknown Region" code, so it is likelier to arrive from a data source as a real value,
  where `XX` is only ever a sentinel here. Either way it is one constant.
- **In `writePublicLookupHeaders`, not `enrich`.** The empty-country case is already this
  function's business; the bug is the word "empty" in its own doc comment. Writing `XX` here
  without setting `written` keeps "first resolved public country wins": chain
  `[unresolved, GB]` still ends `GB`. Substituting `XX` earlier, inside `recordForLookup`,
  marks the country written and locks that chain to `XX`, blocking a legitimate client behind
  a proxy the database does not know.
- **Not `decide`.** Deleting the `country == PrivateIpCountryAlias` branch there restores the
  same verdicts with no new value, but `core_geoblock_plugin_request-mode/spec.md:69` states
  it as a requirement: "`PRIVATE` on `countryHeader` SHALL follow `allowPrivate`." That is a
  spec conflict, not a preference. It would also leave a public address labelled `PRIVATE` in
  logs and on the ban page.

## Risks / Trade-offs

- [Operators with an enabled `bin` row, which is the default] → No change at all. The merged
  country is `-`, never empty, so the new branch never runs.
- [`defaultAllow: false` with no enabled `bin` row, requests blocked that were allowed before]
  → That is the fix. It restores the pre-`mode` outcome and what README documents.
- [`defaultAllow: true` with `allowPrivate: true`] → No change: `XX` matches no country rule
  and falls through to `defaultAllow`, exactly as before.
- [`defaultAllow: true` with `allowPrivate: false`] → An unresolved public IP flips from
  `block:allow_private` to `pass:default_allow`. It was blocked only because it was
  mislabelled private, and `defaultAllow: true` says to allow what no rule matches, so the
  new verdict is the intended one — but it is a change.
- [Clients on CGNAT, link-local or other non-RFC1918 reserved ranges] → `privateOrLoopback`
  uses Go's `IsPrivate`, which is RFC 1918 only, so `100.64.0.0/10` (CGNAT, and Tailscale's
  range), `169.254.0.0/16` and `198.18.0.0/15` were already treated as public. They were
  mislabelled `PRIVATE` and allowed; they now resolve to `XX` and follow `defaultAllow`.
  That matches what README says `allowPrivate` covers, but operators terminating such
  addresses must list them in `allowedIPBlocks` or add `XX` to `allowedCountries`.
- [Downstream consumers of `countryHeader`] → A public address now reports `XX` instead of
  `PRIVATE`. Dashboards keep a populated value, and it is no longer wrong.
- [Consumers of `logStatusDetailHeader`] → The reason for those requests moves from
  `allow_private` to `default_allow`, even where the verdict is unchanged.
- [`CheckAll` with `allowPrivate: true`] → A chain whose earlier selected hop was already
  allowed still passes, because `blockFromHeader` only denies while `passReason` is
  `PhaseNone`. That is separate and older than this change, and is reported separately.

## Migration Plan

Ship it. Operators who want unresolved addresses allowed can add `XX` to `allowedCountries`;
operators who want them denied explicitly can add `XX` to `blockedCountries`. Rollback is
revert.
