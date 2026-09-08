# Make the database download cancelable, so sleep does not wait on it

Size: medium. Found by the Performance axis of the `2026-09-08-reclaim-lifecycle` review.

## What

`dbsource.Updater.Stop` now joins its loop goroutine, which is what makes sleep honest: a stopped
updater cannot write a database file into a directory that is being torn down. But nothing
cancels the HTTP GET that goroutine may be sitting inside. `tick` calls `UpdateIfNeeded` ->
`Update` -> `dbutils.HTTPGet`, and the only bound there is `HTTPGetTimeout = 30 * time.Minute`.

That wait is now reachable from plugin construction. `Table.drop` calls `Sleep` with the slot
parked in `slotBusy`, `BIN.Sleep` and `MMDB.Sleep` call `Updater.Stop`, and any `Open` for that
key parks on `slot.ready` until the sleep returns. So a Traefik dynamic-config reload that lands
in the sleep window stalls for as long as the in-flight download runs.

## Why it was not fixed here

The fix is a cancelable request, and that is a signature change in `pkg/dbutils.HTTPGet` and
`pkg/dbsource.Update` — both outside the diff this ticket owns. The workflow does not apply
cause fixes outside the diff unattended.

## Exposure today

The join is the download's remaining time. It is sub-millisecond whenever no download is in
flight, which is every moment except the few seconds per 24h ticker period per key when one is.
Worst case is the 30-minute client timeout.

Not joining is not an alternative: the unjoined `Stop` is the defect this ticket exists to
remove.

## What to do

Thread a `context.Context` from `Updater` into the request (`http.NewRequestWithContext`) and
cancel it in `Stop`, so the join returns promptly and `tick`'s existing `stopped(stop)` check
before the callback discards whatever partial result comes back.

Related: `pkg/dbutils/httpget.go` documents "Errors and DownloadHint must not include the URL
(tokens may be in the query)" and then wraps `*url.Error` verbatim, which does include it. This
change works around that in `Updater.tick` by stripping the query before logging; the cause is
still in `HTTPGet` and every other caller of it is still exposed.
