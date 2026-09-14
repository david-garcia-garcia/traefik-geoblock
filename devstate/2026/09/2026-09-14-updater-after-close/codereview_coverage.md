# Test coverage

1. [hard] Critical path untested — `pkg/dbwrappers/bin.go:261` — `hotSwap` fail-closed after Close (`closed` then Close/remove copy); `TestOpenBIN_DelayedDownloadAfterClose` still joins and `tick` skips `onUpdate`, so reverting the `closed.Load()` branch leaves Lookup-fail and leftover assertions green
   → Assert `hotSwap` after Close does not publish a live handle and removes the temp copy
   Status: done
   Argument: TestOpenBIN_HotSwapAfterClose in delayed_close_test.go (facb6ac).
2. [hard] Critical path untested — `pkg/dbwrappers/mmdb.go:156` — `swapReader` refuses a non-nil db after Close; `TestOpenMMDB_DelayedDownloadAfterClose` still joins and skips `onUpdate`, so reverting `closed && db != nil` leaves Lookup-fail green (`close` nils the reader after Stop)
   → Assert `open` after Close does not publish a live reader
   Status: done
   Argument: TestOpenMMDB_OpenAfterClose in delayed_close_test.go (facb6ac).
