## MODIFIED Requirements

### Requirement: Enabled rows name a column map
An enabled row MUST set `fields` or `fieldsPreconfigured`, not both. Empty both SHALL fail plugin creation. The failure error MUST state that the plugin does not start and that this middleware is not applied. `fieldsPreconfigured` SHALL be a named preset whose format matches the row `databaseType`. Unknown preset names or a format mismatch SHALL fail plugin creation. After validate, the plugin SHALL expand the preset into `fields` and SHALL clear `fieldsPreconfigured` so a later Prepare is not both-set.

#### Scenario: Unknown preset fails
- **WHEN** an enabled row sets `fieldsPreconfigured` to `not-a-preset`
- **THEN** plugin creation fails

#### Scenario: Preset format mismatch fails
- **WHEN** an enabled row sets `databaseType` to `bin` and `fieldsPreconfigured` to `ipinfo_lite`
- **THEN** plugin creation fails

#### Scenario: Both maps fail
- **WHEN** an enabled row sets `fields` and `fieldsPreconfigured`
- **THEN** plugin creation fails

#### Scenario: Empty both maps names the implication
- **WHEN** an enabled row sets neither `fields` nor `fieldsPreconfigured`
- **THEN** plugin creation fails
- **AND** the error text includes `plugin does not start`
- **AND** the error text includes `this middleware is not applied`
