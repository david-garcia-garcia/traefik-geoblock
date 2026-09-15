# BIN LookupRecord vs hotSwap stays unsynchronized

IssueKey: 2026-09-14-updater-after-close
Size: large
Action: note

## Why this follow-up

BIN `LookupRecord` reads `w.db` while `hotSwap` and `close` assign it, with no mutex. Ignore-after-close only needs a closed flag. The lookup race is ticket `2026-09-14-bin-handle-race`.

## Why it was not taken

Out of scope on this requirement (F-1 / F-1b). Explore showed an `atomic.Bool` plus re-check before publish is enough for ignore-after-close. Unattended take is only small rows on files this run created.

## Risks

Implement might add `sync.RWMutex` around `LookupRecord` while touching `hotSwap`, folding F-1 into this change and widening the blast radius.
