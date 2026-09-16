# Standards

1. [hard] Leave a trail — `pkg/dbwrappers/lifecycle.go:88` — `initialize` comment listed seed-first / dated copy / in-place and omitted the new unreadable-Latest fallback
   → Mention warn + catalog Path then BundledFile when the dated file cannot be opened
   Status: done
   Argument: comment updated in ccab25d.

2. [hard] Leave a trail — `pkg/geoblock/plugin.go:36` — `PhaseNone` still said “no IPs found” after empty hops became `pass:error` / `block:error`
   → Drop that example; name the remaining “no rule after hops” case
   Status: done
   Argument: comment updated in ccab25d.
