# Fix F-4: Updater.Stop does not join an in-flight tick

Fix F-4 from the 2026-09-14 geoblock bug hunt.

`dbsource.Updater.Stop` does not join an in-flight tick. Stop stops the ticker and closes the stop channel, but a tick already inside HTTP download keeps running and calls `onUpdate`, which hot-swaps a wrapper the reclaim table already disposed. The disposed wrapper gets a live file handle, a fresh temp copy in `os.TempDir()`, and successful lookups after Close. The first tick fires immediately at Start, so the window is at startup of every URL-backed wrapper, not only once a day. BIN and MMDB both behave this way.

Files: `pkg/dbsource/updater.go` Start/tick/Stop; `pkg/dbwrappers/bin.go` startUpdate/hotSwap/close; `pkg/dbwrappers/mmdb.go` startUpdate/open/close.

Desired: a disposed generation stays disposed. Stop is synchronous (join the goroutine). tick does not call onUpdate after stop. Wrappers ignore an update that arrives after close. Smallest durable delta; one owner for the join (Updater), wrappers still refuse post-close swap so a missed join cannot reopen.

Do not take F-1's BIN mutex in this ticket unless explore proves the post-close swap cannot be made safe without it — if you must touch BIN hotSwap for the ignore-after-close check, do not expand into the unsynchronized LookupRecord race (that is 2026-09-14-bin-handle-race). Note leftover on issues.md if needed.

Out of scope: F-1 LookupRecord race, F-2 CIDR, F-3 bypass, F-5–F-9. Do not copy `zzz_proof_*` filenames; write proper product tests for the delayed-download-until-after-close scenario (BIN and MMDB).
