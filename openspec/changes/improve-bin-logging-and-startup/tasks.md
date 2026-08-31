## 1. Temp copy name

- [x] 1.1 Name the local copy `bin_<safeKey>_<unixNano>.BIN` in `createLocalCopy`
- [x] 1.2 Assert basename shape (key + numeric nanosecond) when a dated catalog file is copied

## 2. Init and hot-swap logs

- [x] 2.1 Log `source_path` on `BIN initialized` and `BIN hot-swapped`; keep `path` / `new_path` as the opened file
- [x] 2.2 Assert `source_path` is the dated catalog file and `path` is the temp copy

## 3. owner_plugin

- [x] 3.1 Add `logging.NewOwner` (`owner_plugin`, no `plugin`)
- [x] 3.2 Open BIN from catalog with `NewOwner` (middleware name)
- [x] 3.3 Assert `BIN initialized` has `owner_plugin` and does not have `plugin`

## 4. Usage

- [x] 4.1 Update `knowledge/devdocs/core_geoblock_database_wrapper.md` for init attrs and temp basename
