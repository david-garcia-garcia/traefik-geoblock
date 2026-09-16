# Explore
IssueKey: 2026-09-16-zzz-test-prefix

## Concepts

**Package test** (existing Language in `knowledge/devdocs/core_geoblock_test-harness.md`): a `*_test.go` file in the same Go package as the code it covers. The `zzz_` prefix is a basename sort key, not a new unit. Discovery stays the `_test.go` suffix.

**Product test file**: one of the thirty first-party `*_test.go` files on dest (`plugin_instance_test.go` and `pkg/**/*_test.go`). After this change the basename is `zzz_<oldstem>_test.go` (example: `pkg/dbwrappers/bin_test.go` → `pkg/dbwrappers/zzz_bin_test.go`).

**Hunt proof file**: a `zzz_proof_*_test.go` basename used in earlier tickets to mean uncommitted bug-hunt evidence. Dest has none. This change must not introduce those names; `zzz_bin_test.go` is not `zzz_proof_bin_test.go`.

**Owner packages**: module root (`plugin.go` / `plugin_instance_test.go`), `pkg/geoblock`, `pkg/dbwrappers`, `pkg/dbsource`, `pkg/dbprovider`, `pkg/dbutils`, `pkg/fileutils`, `pkg/logging`. Tests stay in those packages. No `tests/` tree.

## Decisions

- Rename by `git mv` every first-party `*_test.go` present at apply so the basename is `zzz_` + the current stem. Keep the `_test.go` suffix, package clause, and `Test*` / `Benchmark*` names. Requirement Desired names this; dest currently has thirty files and no `zzz_` stem (`requirement.md` Affected).
- Do not rename Pester (`scripts/integration-tests.Tests.ps1`), vendor (none), or hunt `zzz_proof_*` files (none on dest; Out of scope).
- Do not rewrite archived OpenSpec paths that cite old basenames (Out of scope).
- Do not rewrite `knowledge/devdocs/core_geoblock_test-harness.md` in apply (Out of scope). `*_test.go` and `pkg/*/*_test.go` globs remain true after the prefix. Stale Key-files stems are a usage-doc follow-up, not this rename.
- No new Language term. Package test stays `*_test.go`. No third-party research write: `go test` and `go mod vendor` already key on the suffix (`core_geoblock_test-harness.md`, `ext_traefik_plugins_yaegi-subpackages/notes.md`).
- No identity owner question: this change does not set or reconstruct client address, user, tenant, Host, or trust hop.

Measured on the worktree (same thirty paths as `requirement.md` Affected; none already `zzz_*`): explorer clustering is a filename-sort gap, not a failing test. Not a runtime failure to reproduce.

```
pkg/dbwrappers/ today          after prefix
  bin.go                       bin.go
  bin_test.go                  fields.go
  fields.go                    ...
  fields_test.go               zzz_bin_test.go
  lifecycle.go                 zzz_fields_test.go
  mmdb.go                      zzz_mmdb_test.go
```

`zzz_` sorts after typical production stems in these packages (`b`–`r`). A later `update.go` would sit after the tests; that is accepted sort cost of the commissioned prefix.

## Open questions

- Q: Must this change rewrite `knowledge/devdocs/core_geoblock_test-harness.md` Key files and how-to-use stems to `zzz_*`?
  Rank: bounded incidental — one usage packet enumerated (`knowledge/devdocs/core_geoblock_test-harness.md` plus `core_geoblock_plugin_instance.md` Key files); requirement Out of scope lists rewriting the harness packet
  Decision: assumed — do not rewrite those packets in apply; honor Out of scope. `*_test.go` globs stay true. Stale explicit stems wait for devdocsimpact / a follow-up note.
  By: explore

- Q: Must a new spec leaf mandate the `zzz_` basename, or does the rename alone satisfy Desired?
  Rank: additive incidental — no existing spec SHALL on test basenames; requirement Desired is the rename, not a new contract folder
  Decision: resolved — rename alone. `skip_specs: true` because plugin and lookup requirements do not change. No spec folder; FindSpecHost not run (no delta to host).
  By: propose

- Q: What if dest gains another first-party `*_test.go` after the dump?
  Rank: additive asked — Desired is every first-party Go package test file; Unknowns already names this
  Decision: resolved — re-listed at apply; thirty files, none new; all prefixed. `vendor/` still has none.
  By: implement
