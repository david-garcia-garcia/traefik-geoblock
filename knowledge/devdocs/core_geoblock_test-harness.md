# Test harness

## Language

**Package test**:
A `*_test.go` file in the same Go package as the code it covers.
_Avoid_: a parallel `tests/` tree; putting `New` / `ServeHTTP` cases under `pkg/`.

**Throughput gate**:
A `TestThroughput_*` that fails if ops/s drop below the floor. Skipped with `-short`.
_Avoid_: treating `Benchmark*` as a CI pass/fail.

**Integration case**:
A Pester `It` against one compose `PathPrefix` and its middleware labels. Client IP is `X-Real-IP` or `X-Forwarded-For`.
_Avoid_: calling `go test` an integration test.

## Overview

Unit and bench tests live next to the owner package. Traefik behavior is proven through Docker Compose plus Pester, not through `httptest` alone.

## How to use

- Change `plugin.go` policy or config → add or extend a package test in the matching `plugin_*_test.go` (config, policy, ipheaders, filters, observe). Shared fixtures stay in `plugin_test.go`.
- Change a helper in `pkg/<name>` → add or extend `pkg/<name>/*_test.go`.
- Change lookup or `ServeHTTP` cost → run `go test -bench=BenchmarkPlugin -benchmem`. Keep `TestThroughput_*` floors conservative; raise them only after CI samples.
- Change Traefik-visible behavior (headers, labels, blocking) → add a `whoami-*` service in `docker-compose.yml` with a unique `PathPrefix` and matching plugin labels, then a Pester `Context` / `It` in `scripts/integration-tests.Tests.ps1`.
- HTTP from Pester: `Invoke-TestRequest`. Access-log asserts: keep the header on the Traefik `accesslog.fields.headers.names.*` command, then `Get-TraefikAccessLogEntries`.
- whoami echoes forwarded request headers in the body (`X-Geo-Country: US`).
- Local: `go test ./...` and `./Test-Integration.ps1`. CI runs `go test -v ./...` (gates included) and the Pester job.

## Pattern snippet

```go
func TestRequestHeaderEnrich(t *testing.T) {
	handler, err := New(context.TODO(), &noopHandler{}, &Config{Enabled: true, /* ... */}, pluginName)
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

- `plugin_test.go` — shared fixtures (`noopHandler`, BIN helpers)
- `plugin_config_test.go` — `New`, deprecated aliases, auto-update
- `plugin_policy_test.go` — country / IP / private allow-deny
- `plugin_ipheaders_test.go` — IP extraction and `ipHeaderStrategy`
- `plugin_filters_test.go` — ignore verbs, include/exclude path regex, bypass headers
- `plugin_observe_test.go` — `logStatusDetailHeader` and `requestHeaderEnrich`
- `perf_test.go` — throughput gates and `BenchmarkPlugin_*`
- `pkg/*/*_test.go` — helper package tests
- `docker-compose.yml` — Traefik + whoami routes
- `scripts/integration-tests.Tests.ps1` — Pester cases
- `Test-Integration.ps1` — compose up, wait, Pester, teardown
- `.github/workflows/ci.yml` — `go test` and integration jobs

## Gotchas

- LITE DB1 has country only. Region/city/ISP/domain/ASN integration asserts must not require values the BIN cannot supply. Paid DB8 lives at `testdata/IP2LOCATION-DB8.BIN` (gitignored); package tests skip if it is absent. ASN needs `IP2LOCATION-LITE-ASN.IPV6.BIN` (or `IP2LOCATION_ASN_BIN`). ASN auto-update does not download without a token.
- Compose labels for the BIN are `ip2location_*` (unprefixed names are deprecated aliases).
- Traefik `forwardedHeaders` must stay on so the plugin sees `X-Real-IP`, not the Docker network address.
- Throughput floors catch large regressions only. Compare impact with benches on the same machine.
