# Review journal

## prepare (2026-09-14)
phase: prepare
findings: none
fixed: none
skipped: CI `-race` overhaul (note large)

## explore (2026-09-14)
phase: explore
findings: none
fixed: none
skipped: CI `-race` overhaul (note large); BIN 10s delayed Close kept (deviation)

## propose (2026-09-14)
phase: propose
findings: none
fixed: none
skipped: CI `-race` overhaul (note large); BIN 10s delayed Close kept (deviation)

## implement (2026-09-14)
phase: implement
findings: none
fixed: BIN RWMutex + swapHandle + LookupRecord RLock/Get_all once; bin_handle_test.go; usage packet
skipped: CI `-race` overhaul (note large); BIN 10s delayed Close kept (deviation)

## codereview (2026-09-14)
phase: codereview
findings: P3 2
fixed: startUpdate comment; hotSwap oldDB renamed to old (04190b9)
skipped: none

## devdocsimpact (2026-09-14)
phase: devdocsimpact
findings: language-gap, stale-usage
fixed: Published handle Language; Path/Version/SourcePath Gotcha
skipped: none

## archive (2026-09-14)
phase: archive
findings: none
fixed: folded wrapper-reclaim and lookup into live specs; moved change to archive/2026-09-14-bin-rwmutex-published-handle
skipped: none

## pullrequest (2026-09-14)
phase: pullrequest
findings: none
fixed: dropped WIP title; CI succeeded on run 34902350749
skipped: none
