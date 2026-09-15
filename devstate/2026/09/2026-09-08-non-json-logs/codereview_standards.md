# Standards

1. [judgement] Mysterious Name — `pkg/logging/logging_test.go:16`, `pkg/geoblock/plugin_config_test.go:948` — `const filedGeoBlockPrefix = "INFO: GeoBlock:"` — `filed` reads like a typo or legal “filed”; the role is the legacy #67 prefix CrowdSec could not parse, which the comment states but the identifier does not
   → Rename to spell the job (e.g. `legacyGeoBlockInfoPrefix` or `geoblockIssue67SamplePrefix`)
   Status: skipped
   Argument: judgement; identifier is test-local and the comment names #67.

2. [judgement] Symmetry and consistency — `pkg/logging/logging_test.go:19`, `pkg/geoblock/plugin_config_test.go:951` — identical stdout pipe-capture helpers are named `captureStdout` in one sibling file and `capturePluginStdout` in the other
   → Use the same identifier for the same role across both test files, or extract one shared helper both call
   Status: skipped
   Argument: judgement; packages cannot share unexported helpers without a new test-only export.

3. [judgement] Symmetry and consistency — `pkg/logging/logging_test.go:389-390`, `pkg/geoblock/plugin_config_test.go:988-1005` — logging subtests call `assertNoFiledPrefix` + `assertTextNotJSONObject`; the geoblock sibling inlines the same prefix and non-JSON line checks in a loop
   → Reuse the same assert pipeline in both files (shared helper or copy the named asserts into geoblock)
   Status: skipped
   Argument: judgement; geoblock assert is one test; extracting a cross-package helper is extra.

4. [judgement] Duplicated Code — `pkg/logging/logging_test.go:19-38`, `pkg/geoblock/plugin_config_test.go:951-970` — near-identical `captureStdout` / `capturePluginStdout` bodies differ only by identifier
   → Extract the shared capture shape once; call it from both packages
   Status: skipped
   Argument: judgement; Bound the ask — do not add a shared test package for two capture helpers.
