## 1. Prepare error implication

- [x] 1.1 Append `; plugin does not start; this middleware is not applied` to the empty-both-maps error in `resolveSourceFields`
- [x] 1.2 Assert that substring in `NeitherFieldsNorPresetFails`

## 2. Wrapper reclaim keys

- [x] 2.1 Change `binKey` to `bin:<Source.Key>:<configHash>`
- [x] 2.2 Change `mmdbKey` to `mmdb:<Source.Key>:<configHash>`
- [x] 2.3 Assert the BIN table key starts with `bin:<catalogKey>:` in the existing hash-change reclaim test

## 3. Usage

- [x] 3.1 Update `knowledge/devdocs/core_geoblock_database_source.md` gotcha with the implication sentence
- [x] 3.2 Update `knowledge/devdocs/core_geoblock_database_wrapper.md` key shape
