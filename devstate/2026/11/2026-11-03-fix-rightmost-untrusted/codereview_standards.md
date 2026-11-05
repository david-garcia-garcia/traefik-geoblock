# Standards

1. [judgement] Duplicated Code — `pkg/geoblock/ip.go:111` — `rightmostChain` copies `collectHops`' `remoteAddress` clean-and-append instead of asking that owner
   Fix: Append `collectHops(req, []string{remoteAddressHeader}, false)...` when `remoteAddress` is listed
   Status: skipped
   Argument: judgement, not applied unattended.
   Quote:
   ```
   ip := cleanIPAddress(req.RemoteAddr)
   if ip == "" {
   	return hops
   }
   return append(hops, ip)
   ```
