## Context

See proposal.md — Why. `countryHeader` is the write/read bridge for country allow/block. `requestHeaderEnrich` already writes every mapped header. `foldCountryHeader` copies `countryHeader` onto enrich when that name is unset, so lookup would write both headers today if `Prepare` did not reject the config.

## Goals / Non-Goals

**Goals:**
- Extra `requestHeaderEnrich` `country` mappings succeed at create and are written on lookup.
- Block continues to read `countryHeader` only.

**Non-Goals:**
- Changing the omitted `countryHeader` default.
- Warning when more than one country header is configured.
- Changing Traefik constructor-failure log fields.

## Decisions

- **Delete `checkCountryHeaderBridge`.** The values are the same lookup country. Alternative: warn and continue — rejected; operator logs already treat Traefik `ERR` as failure, and a warn would still look like a problem.
- **Keep `foldCountryHeader`.** It still ensures `countryHeader` is written when enrich only names the extra header.

## Risks / Trade-offs

- [Operators who thought block read `X-Geo-Country`] → Block still reads `countryHeader` (`X-IPCountry` when omitted). Document that in README and the usage packet.

## Migration Plan

Ship the relaxation. Existing CRDs that map `X-Geo-Country` or `X-IP-Country` to `country` start creating. Rollback is revert.
