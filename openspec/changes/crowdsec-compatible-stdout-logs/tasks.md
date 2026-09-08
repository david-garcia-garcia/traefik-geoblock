## 1. Logging line-shape tests

- [ ] 1.1 In `pkg/logging/logging_test.go`, capture stdout from `NewBootstrap`, `NewOwner`, and `New` (text and json) and assert no `INFO: GeoBlock:` prefix
- [ ] 1.2 Assert text / empty format lines do not trim-start with `{` and `json.Unmarshal` fails
- [ ] 1.3 Assert `logFormat: json` lines trim-start with `{` and `json.Unmarshal` into an object succeeds

## 2. CreateConfig default

- [ ] 2.1 Assert `CreateConfig().LogFormat` is `text`
- [ ] 2.2 Capture `PluginLogger` with `CreateConfig` defaults, write an info line, and apply the same prefix / no-`{` asserts

## 3. Verify

- [ ] 3.1 Run `go test ./pkg/logging/ ./pkg/geoblock/ -count=1` and record `localTests`
