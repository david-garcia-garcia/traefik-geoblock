## 1. Seed-first initialize

- [ ] 1.1 When dated Latest exists and `Search` finds `defaultFile`, open that seed and return before the dated copy
- [ ] 1.2 Copy the dated file off `New` and `hotSwap` when the copy is ready; skip swap after Close
- [ ] 1.3 When `defaultFile` is empty or not found, keep the sync dated copy
- [ ] 1.4 Assert OpenBIN Path is the seed and LookupRecord works before the dated temp copy exists
- [ ] 1.5 Assert no-seed OpenBIN still returns a dated temp copy

## 2. size_bytes

- [ ] 2.1 Log `size_bytes` on `BIN initialized` and `BIN hot-swapped` from `os.Stat` of the opened file
- [ ] 2.2 Assert both lines include `size_bytes` equal to that file’s size

## 3. Usage

- [ ] 3.1 Update `knowledge/devdocs/core_geoblock_database_wrapper.md` for seed-first init and `size_bytes`
