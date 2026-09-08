# Mutation evidence: can these tests fail?

`go test -race` is unavailable on this box (no C toolchain), so every behavioural claim was
checked by breaking the implementation in a signature-preserving way and confirming the named
test goes red. A test that survives its own mutation proves nothing.

Method: back up the file, apply the mutation to a whole-file read (per-line `-replace` silently
no-ops on multi-line patterns — that trap ate one round here), run the package, restore.

## `pkg/reclaim`

| # | Mutation | Caught by |
| --- | --- | --- |
| M1 | `drop` never calls `sleepValue` | `LifecycleIsCreateSleepWakeClose`, `SleepPrecedesCloseAtEveryGrace`, `HolderWithoutDoneChannelIsPolled`, `ResetLogsOrphanThenDispose...`, `ResetStopsGraceWait`, `ConcurrentCancelLastHolders` |
| M2 | `reclaim` never calls `wakeValue` | `LifecycleIsCreateSleepWakeClose`, `OpenWaitsForWake`, `ConcurrentOpensOnSleepingValueWakeOnce` |
| M3 | `dispose` logs `reclaim_dispose` before `Close()` instead of after | `DisposeLogFollowsClose`, `ResetDisposeLogFollowsClose` |
| M4 | zero grace leaves the key mapped after sleep | `ZeroGraceRacingOpenIsPlainBind` |
| M5 | `create` runs before the key is registered (the lost-create race restored) | `ConcurrentFirstOpensCreateOnce`, `CreateErrorReachesEveryWaiter`, `SecondCreateDisposeIgnored`, `ConcurrentOpenSameKeySharesOneIncarnation`, `ReclaimRacesExpiry`, `ResetRacingOpenClosesEveryValue`, `LifecycleIsCreateSleepWakeClose` |

## `pkg/dbsource`

| # | Mutation | Caught by |
| --- | --- | --- |
| M6 | `Stop` does not `running.Wait()` | `StopWaitsForTheUpdateLoop`, `StopThenStartRunsAFreshLoop` |
| M7 | `tick` drops the stop check before the age check | `StoppedLoopDoesNotDownload` |
| M8 | `tick` drops the stop check before the update callback | `StopDuringADownloadSkipsTheCallback` |

M8 was vacuous on the first attempt: the gated test server returned bytes that were not a real
database, so `Update` failed at the build-date read and the callback could never fire either way.
`TestUpdater_RunningTickInvokesTheCallback` is the positive control that now keeps that honest,
and the gated server serves the real seed MMDB.

## Post-review mutations

Added while applying the code review findings, so each fix has a test that fails without it.

| # | Mutation | Caught by |
| --- | --- | --- |
| M9 | `reclaim_orphan` written after the slot leaves `slotBusy` (the Reset window the Spec axis found) | `ResetRacingADropKeepsOrphanBeforeDispose` — round 128 of 400, sequence `put bind dispose orphan` |
| M10 | `Reset` also ends a busy incarnation instead of leaving it to the goroutine that owns it | `ResetDuringSleepStillOrphansBeforeDispose` |
| M11 | every `Open` takes the slot logger, including one that never binds | `AnOpenThatOnlyWaitsDoesNotTakeTheLogger` |
| M12 | `tick` logs the raw transport error | `FailedDownloadDoesNotLogTheToken` — printed the live token |
| M4b | zero grace leaves the key mapped, against the rewritten deterministic test | `ZeroGraceRacingOpenIsPlainBind` |

M4's original test did catch M4, contrary to the Coverage axis's reasoning, but it caught it by
winning a race. The rewrite holds the value inside `Sleep()` so the window is entered every run.

## Contention run

`GOMAXPROCS=2` with 12 hidden `Start-Process` CPU burners.

Before the review fixes:

- `go test ./pkg/reclaim/ -count=60` — ok, 44.8s
- `go test ./pkg/dbwrappers/ ./pkg/dbsource/ -count=10` — ok, 23.0s and 24.8s

After:

- `go test ./pkg/reclaim/ -count=40` — ok, 35.6s
- `go test ./pkg/dbwrappers/ ./pkg/dbsource/ -count=8` — ok, 10.6s and 9.6s