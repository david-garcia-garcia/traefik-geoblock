# Test coverage

1. [judgement] Happy path only — `pkg/dbwrappers/lifecycle.go:108` — BIN/MMDB/New tests prove seed publish; they do not assert the warn names the dated file
   → Assert the warn line, or skip: startup success + live seed is the ticket job
   Status: skipped
   Argument: judgement; seed-open and Traefik New already fail on revert; warning text is not a deny path.
