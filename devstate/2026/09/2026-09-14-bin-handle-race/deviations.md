# Deviations

- [x] taken  keep BIN 10s delayed Close after swap instead of MMDB immediate Close
  Asked: BIN mutex discipline matching MMDB.
  Instead: `hotSwap` still delayed-Closes the previous `*ip2loc.DB` after 10s.
  Owner: `pkg/dbwrappers/bin.go`
  Why: mutex already waits in-flight `Get_all` before swap; the 10s delay is existing BIN behavior and no criterion names it.
  By: explore
  Requester: not asked
