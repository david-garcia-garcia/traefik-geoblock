## 1. Reclaim table: four-event lifecycle and the new slot shape

- [x] 1.1 Add `sleeper` and `waker` optional interfaces next to `closer`, plus the helpers that assert them, mirroring `stopValue`'s shape.
- [x] 1.2 Replace the `slot` fields: drop `cancel`, `graceTimer`, `graceGen`, `holders map[uint64]struct{}` and `nextID`; add `err`, `state`, `ready`, `woken`, and an integer holder count.
- [x] 1.3 Rewrite `Open` to register the key in `busy` before running `create`, park a second caller on `ready`, and re-loop; return the create error to every parked caller.
- [x] 1.4 Add the `asleep` branch of `Open`: cancel the grace wait, run `Wake` outside the mutex, then log `reclaim_reclaim` and `reclaim_bind`.
- [x] 1.5 Rewrite `drop` so the watcher goroutine runs `Sleep`, logs `reclaim_orphan`, waits out grace on a `select` over the timer and `woken`, calls `Close`, and logs `reclaim_dispose`.
- [x] 1.6 Delete the per-slot lifetime context, its `cancel`, the parked goroutine, and the `fire`/`bindLocked`/`logBind` shape that only existed to support them.
- [x] 1.7 Make the zero-grace path unmap the key in the same critical section that leaves `busy`, so no `Open` can observe a sleeping value at zero grace.
- [x] 1.8 Rewrite `Reset` to sleep an awake incarnation before closing it, log `reclaim_orphan` then `reclaim_dispose`, and release grace waits through `woken`.
- [x] 1.9 Update the `Table` doc comment and state diagram to the four states, and the `Open`/`Reset` doc comments to the new contract.

## 2. Reclaim table tests

- [x] 2.1 Carry over the master tests that still express true requirements; delete `TestTable_LostCreateRaceCancelsLoser` and the three `StaleFire*` tests, whose mechanism is gone.
- [x] 2.2 Add lifecycle-order tests: create/sleep/wake/close sequence over repeated cycles, one `sleep` per orphan, no double sleep at grace expiry.
- [x] 2.3 Add a test that `Open` does not return while `Wake` is blocked, and that concurrent `Open`s on a sleeping key wake it once.
- [x] 2.4 Add a test that concurrent first `Open`s run `create` exactly once and discard no value, and that a create error reaches every parked caller.
- [x] 2.5 Add ordering tests: `reclaim_orphan` before `reclaim_dispose` at a sub-millisecond grace over many rounds; `reclaim_dispose` only after `Close()` returned; the same on the `Reset` path.
- [x] 2.6 Add the zero-grace test that a racing `Open` records `reclaim_put`/`reclaim_bind` and not `reclaim_reclaim`.
- [x] 2.7 Add a goroutine-baseline test over many keys, and keep `TestTable_StdlibImports` passing.
- [x] 2.8 Keep a value that implements no lifecycle method working end to end, and a value that implements only `Close`.

## 3. `pkg/dbsource.Updater`: joinable and restartable

- [x] 3.1 Add a mutex and a `sync.WaitGroup` to `Updater`; make `Start` refuse to double-start and the loop goroutine defer `Done`.
- [x] 3.2 Make `tick` check the stop signal before the age check and again before the update callback.
- [x] 3.3 Make `Stop` stop the ticker, close the signal once, wait for the goroutine, then clear the ticker and signal so a later `Start` is clean.
- [x] 3.4 Test that `Stop` does not return while a tick is in flight, that a stopped updater performs no download or callback, that stop-then-start runs one fresh loop, and that stopping twice is safe.

## 4. `pkg/dbwrappers`: BIN and MMDB sleep and wake

- [x] 4.1 Add a mutex to `BIN` over `db`/`path`/`version`/`updater`, matching the `RWMutex` `MMDB` already has.
- [x] 4.2 Implement `BIN.Sleep`/`BIN.Wake` (stop and join the updater, restart it) and reduce `BIN.Close` to releasing the database handle.
- [x] 4.3 Implement `MMDB.Sleep`/`MMDB.Wake` with the same shape and reduce `MMDB.Close` the same way.
- [x] 4.4 Update `pkg/dbwrappers` tests: an orphaned wrapper stops its update loop, a woken wrapper serves lookups and updates again, identity is preserved, and disposal does not stop the loop twice. Wait for `reclaim_orphan` before asserting a reclaim.

## 5. Docs, verification, and follow-ups

- [x] 5.1 Update `knowledge/devdocs/std_go_reclaim.md`: add the sleep/wake language, redefine Grace, remove the Lifetime term, and add the gotchas this change creates.
- [x] 5.2 Record the "release the wrapper database handle on sleep" follow-up as `knowledge/debt/` plus a `devstate/issues.md` row.
- [x] 5.3 Run `go build ./...` and `go vet ./...`, then the full suite, then `pkg/reclaim` under CPU contention with `GOMAXPROCS=2` and a high `-count`.
- [x] 5.4 Verify formatting with `gofmt -d` on LF copies outside the repo (the checkout is `core.autocrlf=true`), and run `openspec validate --strict reclaim-value-sleep-wake-lifecycle`.
