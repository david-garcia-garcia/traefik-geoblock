# Nitpicks

1. [hard] Symmetry and consistency — `pkg/dbwrappers/bin.go:260` vs `pkg/dbwrappers/bin.go:374` — `hotSwap` names the previous vendor handle `oldDB`; sibling `close` (and `swapHandle`) name the same role `old`
   → Rename `oldDB` to `old` so hotSwap, close, and swapHandle share the identifier (MMDB `swapReader` / `close` already use `old`)
   Status: done
   Argument: renamed hotSwap oldDB to old in pkg/dbwrappers/bin.go (04190b9).
