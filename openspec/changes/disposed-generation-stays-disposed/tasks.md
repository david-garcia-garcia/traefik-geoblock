## 1. Updater join

- [ ] 1.1 In `pkg/dbsource/updater.go`, add a `done` channel: the ticker goroutine closes it on exit; `Stop` closes `stop` then waits on `done`. Keep nil-receiver and double-`Stop` safe. Do not add `context` to `HTTPGet`.
- [ ] 1.2 In `tick`, after `UpdateIfNeeded` returns, if `stop` is signaled, return without calling `onUpdate`.
- [ ] 1.3 Add `pkg/dbsource` coverage that `Stop` joins a tick still inside a held GET and that `onUpdate` is not called after that `Stop`.

## 2. Wrapper ignore-after-close

- [ ] 2.1 BIN: `atomic.Bool` closed flag set true before `Stop` in `close`. `hotSwap` re-checks immediately before assigning `w.db`. If closed after a copy was opened, close that DB and remove the temp file. Do not add `sync.RWMutex` around `LookupRecord`.
- [ ] 2.2 MMDB: closed flag under the existing `mu`, set true before `Stop` in `close`. `open` must not publish a reader when closed. Do not treat `db == nil` plus empty path as closed.

## 3. Product tests

- [ ] 3.1 `TestOpenBIN_DelayedDownloadAfterClose` in `pkg/dbwrappers` (ordinary `_test.go`): hold the GET until after Close, then release. Assert Close waits for the ticker, Lookup fails, and no late `bin_*` temp copy remains. Not `zzz_proof_*`.
- [ ] 3.2 `TestOpenMMDB_DelayedDownloadAfterClose` in `pkg/dbwrappers` (ordinary `_test.go`): same delayed-download-until-after-close for MMDB. Assert Close waits and Lookup fails. Not `zzz_proof_*`.
