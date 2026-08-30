## 1. Config

- [x] 1.1 Replace `Enabled` with `Mode` (`disabled` | `enrich` | `block` | `enrichandblock`). Empty is `disabled`. Unknown fails `Prepare`. Delete `enabled`.
- [x] 1.2 Require `countryHeader` when mode is not `disabled`. Fail if another `requestHeaderEnrich` header maps to `country`.
- [x] 1.3 `Prepare`: skip catalog defaults / auto-update dir when mode is `disabled` or `block`. Validate `IPHeaders` when mode is not `disabled`.

## 2. Request path

- [x] 2.1 `NewCore` / `bindDatabase`: open DatabaseProvider only for `enrich` and `enrichandblock`.
- [x] 2.2 Lookup stage writes `countryHeader` + `requestHeaderEnrich`. Block stage reads `countryHeader` for country rules. Do not pass `Record.Country` into country maps.
- [x] 2.3 `block` does not call `setPrivateGeoHeaders`. CIDR / private / `IPHeaders` still run. Missing / `null` country uses `banIfError`. `PRIVATE` follows `allowPrivate`.
- [x] 2.4 `disabled` passes through with no lookup and no block.

## 3. Tests and docs

- [x] 3.1 Package tests: each mode; block without a provider; countryHeader required; block preserves inbound country; CIDR in block; missing header + `banIfError`.
- [x] 3.2 Replace `Enabled: true` in tests with `mode: enrichandblock` and a `countryHeader`.
- [x] 3.3 README: `mode`, required `countryHeader`, CheckAll country narrowing and alternates, enrich-then-block example.
- [x] 3.4 Compose: `mode` + `countryHeader` on existing whoami middlewares. `go test ./...`.
