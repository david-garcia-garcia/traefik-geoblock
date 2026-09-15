# Deviations

- [x] taken  keep BIN 10s delayed Close after swap instead of MMDB immediate Close
  Asked: BIN mutex discipline matching MMDB.
  Instead: `hotSwap` still delayed-Closes the previous `*ip2loc.DB` after 10s.
  Owner: `pkg/dbwrappers/bin.go`
  Why: mutex already waits in-flight `Get_all` before swap; the 10s delay is existing BIN behavior and no criterion names it.
  By: explore
  Requester: not asked

- [x] taken  no request-path mutex on BIN; stale Path/Version/SourcePath accepted
  Asked: BIN mutex discipline matching MMDB; RLock across Get_all; locked getters.
  Instead: lookup copies `w.db` once with no lock; close Closes and does not nil the pointer; Path/Version/SourcePath may be stale during hot-swap.
  Owner: `pkg/dbwrappers/bin.go`
  Why: a lock on every Get_all is request-path cost the owner rejected; a nil pointer is the panic; torn siblings are an accepted inconsistency.
  By: implement
  Requester: confirmed
