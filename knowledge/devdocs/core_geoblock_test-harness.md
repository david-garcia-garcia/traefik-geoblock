# Test harness

## Language

**Package test**:
A `*_test.go` file in the same Go package as the code it covers.
_Avoid_: a parallel `tests/` tree; putting instance-reclaim cases in `pkg/geoblock`.

**Throughput gate**:
A `TestThroughput_*` that fails if ops/s drop below the floor. Skipped with `-short`.
_Avoid_: treating `Benchmark*` as a CI pass/fail.

**Integration case**:
A Pester `It` against one compose `PathPrefix` and its middleware labels. Client IP is `X-Real-IP` or `X-Forwarded-For`.
_Avoid_: calling `go test` an integration test.

## Overview

Unit and bench tests live next to the owner package. Traefik behavior is proven through Docker Compose plus Pester, not through `httptest` alone.

## How to use

- Change `pkg/geoblock` policy or config → add or extend a package test in `pkg/geoblock/plugin_*_test.go` (config, policy, ipheaders, filters, observe). Shared fixtures stay in `pkg/geoblock/plugin_test.go`.
- Change instance reclaim (name+config key, grace) → add or extend `plugin_instance_test.go` at module root. That file calls root `New`, not `newTestPlugin`.
- Change a helper in `pkg/<name>` → add or extend `pkg/<name>/*_test.go`.
- Change lookup or `ServeHTTP` cost → run `go test -bench=BenchmarkPlugin -benchmem`. MaxMind benches (`BenchmarkPlugin_*MaxMind`) use dummy IP `81.2.69.142`, not 8.8.8.8. Keep `TestThroughput_*` floors conservative; raise them only after CI samples.
- Change Traefik-visible behavior (headers, labels, blocking) → add a `whoami-*` service in `docker-compose.yml` with a unique `PathPrefix` and matching plugin labels, then a Pester `Context` / `It` in `scripts/integration-tests.Tests.ps1`. Shared-middleware incarnation + config-change grace is `/reclaima` `/reclaimb` `/reclaimc` and the Pester Context `Shared middleware incarnation and config change`.
- HTTP from Pester: `Invoke-TestRequest`. Access-log asserts: keep the header on the Traefik `accesslog.fields.headers.names.*` command, then `Get-TraefikAccessLogEntries`.
- whoami echoes forwarded request headers in the body (`X-Geo-Country: US`).
- Local: `go test ./...`, `golangci-lint run` (CI is action v6 / golangci-lint v1 — `docker run --rm -v "${PWD}:/app" -w /app golangci/golangci-lint:v1.64.8 golangci-lint run --timeout 5m`), and `./Test-Integration.ps1`. Do not treat a fast desktop `go test` as CI: `TestThroughput_*` floors must also pass on linux/Go 1.21 (CI image). CI runs Lint, `go test -v ./...` (gates included), and the Pester job.
- Token-protected auto-update (paid IP2Location, ASN LITE, IPinfo Lite): copy `.env.example` to `.env` (gitignored). `Test-Integration.ps1` enables compose profile `local-tokens` when `IP2LOCATION_DOWNLOAD_TOKEN` is set. Pester Context `Token-protected database download` skips unless the matching token is set; it asserts Traefik log lines and must not print log bodies (URLs can carry a token).

## Pattern snippet

```go
func TestRequestHeaderEnrich(t *testing.T) {
	handler, err := New(context.TODO(), &noopHandler{}, &Config{Mode: geoblock.ModeEnrichAndBlock, CountryHeader: "X-Country", /* ... */}, pluginName)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}
```

```yaml
- "traefik.http.routers.router-enrichTest.rule=PathPrefix(`/enrichTest`)"
- "traefik.http.middlewares.geoblock-enrichTest.plugin.geoblock.requestHeaderEnrich.X-Geo-Country=country"
```

## Key files

- `plugin.go` — Yaegi entrypoints and instance reclaim
- `plugin_instance_test.go` — root `New` share / miss / reclaim / grace
- `pkg/geoblock/plugin_test.go` — shared fixtures (`noopHandler`, BIN helpers)
- `pkg/geoblock/plugin_config_test.go` — `New`, deprecated aliases, auto-update
- `pkg/geoblock/plugin_policy_test.go` — country / IP / private allow-deny
- `pkg/geoblock/plugin_ipheaders_test.go` — IP extraction and `ipHeaderStrategy`
- `pkg/geoblock/plugin_filters_test.go` — ignore verbs, include/exclude path regex, bypass headers
- `pkg/geoblock/plugin_observe_test.go` — `logStatusDetailHeader` and `requestHeaderEnrich`
- `pkg/geoblock/perf_test.go` — throughput gates and `BenchmarkPlugin_*`
- `pkg/*/*_test.go` — helper package tests
- `docker-compose.yml` — Traefik + whoami routes
- `scripts/integration-tests.Tests.ps1` — Pester cases
- `Test-Integration.ps1` — compose up, wait, Pester, teardown
- `.env.example` — local download tokens; copy to `.env` (not committed)
- `.golangci.yml` — enabled linters (gofmt, gosec, goconst, unparam, predeclared, …)
- `.github/workflows/ci.yml` — Lint, `go test`, and integration jobs

## Gotchas

- LITE DB1 has country only. Region/city/ISP/domain/ASN integration asserts must not require values the BIN cannot supply. Paid DB8 lives at `testdata/IP2LOCATION-DB8.BIN` (gitignored); package tests skip if it is absent. ASN needs `IP2LOCATION-LITE-ASN.IPV6.BIN` (or `IP2LOCATION_ASN_BIN`). ASN auto-update does not download without a token.
- IPinfo Lite snapshot is `seeds/ipinfo_lite.mmdb`. Provider tests and `/ipinfo` compose use it. `/ipinfo` enriches every Lite column (country, country_name, continent, continent_code, isp, domain, asn). ASN strings are `AS…`.
- MaxMind dummy seed is `seeds/GeoIP2-Country-Test.mmdb`. `/maxmind` compose uses dummy IPs `81.2.69.142` (GB) and `175.16.199.1` (CN), not 8.8.8.8.
- Compose seed labels are `databaseSources.<key>.path` plus the vendor pointer (`ip2location_source_geo`, `ipinfo_source`, `maxmind_source`).
- Traefik `forwardedHeaders` must stay on so the plugin sees `X-Real-IP`, not the Docker network address.
- Throughput floors catch large regressions only. Compare impact with benches on the same machine.
