## 1. Rename

- [ ] 1.1 List first-party `*_test.go` excluding `vendor/` and any basename that already starts with `zzz_`
- [ ] 1.2 `git mv` each listed file to `zzz_<oldstem>` in the same directory (example: `bin_test.go` → `zzz_bin_test.go`)
- [ ] 1.3 Do not rename `scripts/integration-tests.Tests.ps1`, vendor paths, or add `zzz_proof_*` files

## 2. Verify

- [ ] 2.1 Run `go test ./...` and confirm the same packages still have tests
- [ ] 2.2 Do not edit test function bodies, package clauses, or usage packets
