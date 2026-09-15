# Test coverage

1. [hard] Critical path untested — `pkg/geoblock/ip_blocks.go:21` — invalid static CIDR fails the load (`AddCIDR` error returns); tests only cover valid static lists
   → Assert `loadIPBlockHelper([]string{"not-a-cidr"}, "", logger)` returns an error
   Status: done
   Argument: TestLoadIPBlockHelper_InvalidStaticCIDR.
2. [hard] Edge case untested — `pkg/geoblock/ip_blocks.go:58` — non-`.txt` files are skipped; no test hits that filter
   → Assert a directory that holds only a non-`.txt` file with a CIDR does not match that IP
   Status: done
   Argument: TestLoadIPBlockHelper_SkipsNonTxt.
3. [judgement] Happy path only — `pkg/geoblock/ip_blocks.go:53` — walk access error and read-file error warn and continue; untested
   → Assert a walk or read failure is skipped and other `.txt` files still load, or skip if unreachable
   Status: skipped
   Argument: judgement; walk/read failures are warn-and-continue leftovers from DestBranch, not this change's ticket job.
