# Yaegi plugin generics

Traefik `v3.7.11` / Yaegi `v0.16.1`. A generic `Table[T]` **defined in another package** cannot be named or held at package scope in the plugin (`*reclaim.Table[*BIN]`). Same-package generics, and a non-generic `any` box in another package plus a type-assert in the caller, both load and run.

## What fails (cross-package `Table[T]`)

Throwaway plugin `yaegi-generic-probe`. `pkg/reclaim` has `type Table[T any]`.

| Shape in the root package | Result |
|---|---|
| Local `dbs := reclaim.NewTable[*db]()` inside `New` | Works (`GENERIC-PROBE same=true id=7`) |
| `var dbs = reclaim.NewTable[*db]()` | Import error: `constant definition loop` |
| `var dbs *reclaim.Table[*db]` or `func() *reclaim.Table[*db]` | Yaegi **panic** `nodeType2` (`yaegi@v0.16.1/interp/type.go:1083`) |
| `var dbs any` then `dbs.(*reclaim.Table[*db])` | Import error: `type not found: reclaim.Table[*db]` |

The broken thing is **instantiating a generic type from package A with a type argument from package B** as a named package-level type. Not “Yaegi cannot run any generics.”

Go has no inheritance. The usual stand-ins still name that type in the consumer package and die the same way (2026-08-28, same image):

| Shape in the root package | Result |
|---|---|
| `type dbTable = reclaim.Table[*db]` + `var dbs *dbTable` | Panic `nodeType2` |
| `type dbTable struct { *reclaim.Table[*db] }` + `var dbs *dbTable` | Panic `nodeType2` |

## What works (2026-08-28 matrix, same image)

| # | Shape | Result |
|---|---|---|
| 01 | `map[string]*db` in the root package | PASS |
| 02 | `var slot any` in the root package, `slot.(*db)` | PASS |
| 03 | `box.Store.Any` (other package) = `*db`, assert in root | PASS |
| 04 | other-package interface, assert `*db` in root | PASS |
| 05 | other-package interface, method call only | PASS |
| 06 | `unsafe.Pointer` in other package, convert in root (`useUnsafe`) | PASS |
| 07 | `type Table[T any]` **and** `var dbs *Table[*db]` in the **same** package | PASS |
| 08 | non-generic `table` with `map[string]*db` in the root package | PASS |
| 09 | other-package interface, assert a local `hasID` interface | PASS |
| 12 | other-package `map[string]any` `Put`/`Get`, `Get("k").(*db)` in root | PASS |

03 and 12 were re-checked from a fresh container log: `GENERIC-PROBE same=true id=7`. 10/11 still fail as in the first table.

The `InstanceLock` comment (“Yaegi panics on a type-assert of a value stored as `any` in this package”) is **not true** on this host for a field or `map[string]any` in a subpackage. The panic we hit is generic instantiation across packages, not `any`.

Host: compose `traefik:v3.7.11` (built 2026-08-19), local plugin GOPATH, `useUnsafe=true` on the unsafe case only. Extracts: [`.sources/compose-v3.7.11-table-t-probe.md`](.sources/compose-v3.7.11-table-t-probe.md), [`.sources/compose-v3.7.11-workaround-matrix.md`](.sources/compose-v3.7.11-workaround-matrix.md).

## Workarounds that give one store

1. **Non-generic reclaim map of `any` + dispose + leases.** Caller type-asserts in `dbwrappers` (`v.(*BIN)`). Measured (12). Reclaim stays stdlib, no `T`.
2. **`Table[T]` lives in `pkg/dbwrappers`**, next to `*BIN` / `*MMDB` (`var bins *Table[*BIN]`). Measured (07). Not a separate copy-paste package unless another repo copies the file into the package that owns `T`.
3. **Two concrete non-generic tables** in `dbwrappers` (08). No `any`, no generics. Duplicate `Open` for BIN and MMDB.

Do not put `Table[T]` in `pkg/reclaim` and name `*reclaim.Table[*BIN]` in `dbwrappers`.
