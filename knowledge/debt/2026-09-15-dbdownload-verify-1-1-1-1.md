# `tools/dbdownload` verify still requires `1.1.1.1` → `US`

IssueKey: 2026-09-15-update-seed-databases
Size: large
Action: note

## Why this follow-up
`verifyDatabase` refuses to write unless `Get_country_short("1.1.1.1")` is `US`. Official IP2Location LITE (committed seed and the 2026-09-15 extract) both return `AU`. `plugin.go` `go:generate` therefore cannot refresh `seeds/`.

## Why it was not taken
Requirement Out of scope: do not change the verify rule unless the official LITE ZIP or BIN name moved. Neither did. Unattended take is only small rows on files this run created.

## Risks
A later `go generate` or `go run ./tools/dbdownload` keeps failing on a valid official LITE. Operators may treat that as a corrupt download.

## Context
Current: `tools/dbdownload/main.go` `verifyDatabase` sentinel `1.1.1.1` / `US`.
Policy and IPinfo tests already treat `1.1.1.1` as AU.
`8.8.8.8` remains `US` on both the committed and the newer LITE.
