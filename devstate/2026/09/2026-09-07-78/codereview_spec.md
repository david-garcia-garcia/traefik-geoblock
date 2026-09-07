# Spec

1. [wrong] openspec/changes/bin-country-short-empty/specs/core_geoblock_database_lookup/spec.md — Requirement: BIN Lookup applies mapped Get_all columns — invalid strings SHALL become empty on the Record
   Status: done
   Argument: narrowed the SHALL so dest `LookupRecord` invalid-as-error stays (`banIfError`); ticket only asked `-` through `usableMeta`.
