# Update test-harness Key files to `zzz_*_test.go`

IssueKey: 2026-09-16-zzz-test-prefix
Size: large
Action: note

## Why this follow-up
`knowledge/devdocs/core_geoblock_test-harness.md` and `knowledge/devdocs/core_geoblock_plugin_instance.md` Key files still cite unprefixed stems after first-party tests were renamed to `zzz_<stem>_test.go`.

## Why it was not taken
Requirement listed rewriting the harness packet under Out of scope. Explore assumed honor that. Unattended take is only small rows on files this run created.

## Risks
Later agents add tests at the old basename and the explorer mix returns.

## Context
Current: `plugin_instance_test.go`, `pkg/geoblock/plugin_*_test.go`
Proposed: `zzz_plugin_instance_test.go`, `pkg/geoblock/zzz_plugin_*_test.go`
