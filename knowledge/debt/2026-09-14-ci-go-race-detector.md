# Enable the Go race detector in CI

IssueKey: 2026-09-14-bin-handle-race
Size: large
Action: note

## Why this follow-up
`.github/workflows/ci.yml` runs `go test -v ./...` and `Makefile` runs `go test -v -cover .`. Neither passes `-race`, so BIN dispose/hot-swap races stay invisible even after lifecycle tests exist.

## Why it was not taken
This ticket is the BIN mutex fix. Adding `-race` is not a one-line flag that already works: `pkg/logging/logging_test.go` races a capture buffer (`TestStdoutWriter_Write`, `TestNew_JSONFormat`), and Yaegi tests skip under `-race`. That is a CI/test-harness job, not this wrapper change. GitHub `ubuntu-latest` already has a C toolchain, so missing gcc is not the blocker.

## Risks
Lifecycle races of the F-1 class can land again without a CI signal. A naive `-race` flip will fail on logging tests first.

## Context
Current: `.github/workflows/ci.yml` `go test -v ./...`; `Makefile` `go test -v -cover .`
Proposed: a dedicated `-race` job or target after logging tests are race-clean.
