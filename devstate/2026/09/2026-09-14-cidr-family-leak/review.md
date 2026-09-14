## prepare (2026-09-14)
phase: prepare
findings: none
fixed: none
skipped: none
qualify: qualified-with-gaps
pr: 86
ci: build 34897940802 in progress

## explore (2026-09-14)
phase: explore
findings: none
fixed: none
skipped: F-5 `/0` vs longest-prefix in `decide` (not inseparable; debt note)
pr: 86
ci: build 34898689824 in progress
open-questions: 5 (3 assumed, 2 resolved)
escalation: none

## propose (2026-09-14)
phase: propose
findings: none
fixed: none
skipped: none
change: cidr-family-isolation
spec: core_geoblock_iplookup_family-match (new)
pr: 86
ci: build 34899411969 in progress
open-questions: 5 (3 assumed, 2 resolved)

## implement (2026-09-14)
phase: implement
findings: none
fixed: two family trees on IpLookupHelper; colliding-prefix product tests; README family isolation
skipped: F-5 decide /0 sentinel (debt note)
pr: 86
ci: build 34900132878 Test failed, Lint and Integration Tests succeeded
localTests: passed
open-questions: 5 (3 assumed, 2 resolved)

## codereview (2026-09-14)
phase: codereview
findings: none
fixed: none
skipped: none
pr: 86
ci: build 34901260180 Lint/Test/Integration success
localTests: passed
open-questions: 5 (3 assumed, 2 resolved)
reviewed-head: 423d837

## devdocsimpact (2026-09-14)
phase: devdocsimpact
findings: none
fixed: none
skipped: none
produced: 0
skipped-findings: 0
pr: 86
ci: build 34902064718 Lint/Test/Integration success
localTests: passed
open-questions: 5 (3 assumed, 2 resolved)
reviewed-head: 8def523

## archive (2026-09-14)
phase: archive
findings: none
fixed: synced core_geoblock_iplookup_family-match into live catalog; moved change to openspec/changes/archive/2026-09-14-cidr-family-isolation
skipped: none
pr: 86
ci: build 34902799987 in progress
localTests: passed
open-questions: 5 (3 assumed, 2 resolved)
reviewed-head: ac994c6
archived: openspec/changes/archive/2026-09-14-cidr-family-isolation

## pullrequest (2026-09-14)
phase: pullrequest
findings: none
fixed: dropped WIP title on PR 86
skipped: comments.md publish (file absent)
pr: 86
title: 🐛 fix(iplookup): match CIDR allow and block lists by address family
ci: build 34903014575 Lint/Test/Integration success
localTests: passed
open-questions: 5 (3 assumed, 2 resolved)
reviewed-head: 36b8d1b
verdict: ready for review
