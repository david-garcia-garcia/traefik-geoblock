## ADDED Requirements

### Requirement: Stdout log lines stay CrowdSec-safe
Plugin stdout log lines SHALL NOT use the prefix `INFO: GeoBlock:`. When `logFormat` is `text` or omitted, each emitted line SHALL NOT begin with `{` after optional leading space. When `logFormat` is `json`, each emitted line SHALL be a JSON object (`json.Unmarshal` into an object succeeds). `CreateConfig` SHALL keep default `logFormat` as `text`.

#### Scenario: Default logger never emits the filed prefix
- **WHEN** the plugin stdout logger is created from `CreateConfig` defaults and an info line is written
- **THEN** the captured stdout does not contain `INFO: GeoBlock:`
- **AND** the line after trim does not start with `{`

#### Scenario: Text format is not JSON
- **WHEN** `logFormat` is `text` and an info line is written
- **THEN** `json.Unmarshal` of that line fails
- **AND** the line after trim does not start with `{`

#### Scenario: JSON format is a JSON object
- **WHEN** `logFormat` is `json` and an info line is written
- **THEN** `json.Unmarshal` of that line into an object succeeds
- **AND** the line after trim starts with `{`
