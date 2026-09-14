# Standards

1. [hard] Leave a trail — `pkg/dbwrappers/bin.go:227` — edited `startUpdate` has no job comment (siblings `sleep` / `wake` do)
   → Add a succinct comment that it starts the keep-current ticker which hot-swaps a newer dated BIN
   Status: done
   Argument: commented startUpdate in pkg/dbwrappers/bin.go (04190b9).
