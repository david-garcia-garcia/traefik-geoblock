# Port the reclaim four-event lifecycle to traefik-modsecurity

IssueKey: 2026-09-08-reclaim-lifecycle
Size: large
Action: note

## Why this follow-up

`pkg/reclaim` is one implementation kept in two repositories and synced in both directions. Before
this change the two copies differed only in local variable names. This change rewrites the table's
internals and adds two optional interfaces on the stored value, so the copies now describe
different behaviour: an unheld value sleeps here and stays fully live there. Leaving that gap open
means the next sync in either direction silently reverts one side.

## Why it was not taken

The port is work in a different repository, with its own tests, its own CI, and its own consumers
of the table — `traefik-modsecurity` has no `pkg/dbsource.Updater` and no GeoIP wrappers, so what
`Sleep` should release there is a separate question. An unattended run does not open a pull request
against a second repository it was not pointed at.

## Risks

A future edit to either copy is now a merge rather than a file copy. If the modsecurity side is
patched first and copied over, the four-event lifecycle disappears without a failing test to say
so, because that repository has no test for it.

## Context

Current: `david-garcia-garcia/traefik-modsecurity` `pkg/reclaim/{table.go,default.go,table_test.go}`
match this repository's `origin/master` modulo local names.
Proposed: copy this change's `table.go`, `default.go`, and `table_test.go` across, and decide there
whether any stored value implements `Sleep`/`Wake`.
