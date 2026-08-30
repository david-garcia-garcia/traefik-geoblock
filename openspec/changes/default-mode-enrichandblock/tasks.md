## 1. Default

- [ ] 1.1 `CreateConfig` sets `Mode` to `enrichandblock`.
- [ ] 1.2 `NormalizeMode` maps empty and whitespace-only `mode` to `enrichandblock`.
- [ ] 1.3 Update `Config.Mode` and `NormalizeMode` comments.

## 2. Tests

- [ ] 2.1 Assert `CreateConfig().Mode` is `enrichandblock`.
- [ ] 2.2 Assert empty and whitespace `mode` prepare/create as `enrichandblock` (opens catalog).
- [ ] 2.3 Keep explicit `mode: disabled` as pass-through.
- [ ] 2.4 Fix tests that constructed empty `Mode` and expected disabled or skipped lookup setup.

## 3. Docs

- [ ] 3.1 README: empty `mode` is `enrichandblock`.
- [ ] 3.2 Usage packet `knowledge/devdocs/core_geoblock_plugin_request-mode.md`: Language and How to use.
