# Test coverage

1. [hard] Edge case untested — `pkg/geoblock/ip.go:134` — unparsable hop stays the evaluated hop (`ParseIP` nil is not trusted); no test fails if that branch treats it as trusted or as no client IP
   Fix: Assert `X-Forwarded-For: not-an-ip, 10.10.60.5` with trusted `10.10.60.0/24` evaluates `not-an-ip`
   Status: done
   Argument: 3dee37b added TestRightmostUntrusted_UnparsableHopIsEvaluated.
   Quote:
   ```
   ipAddr := net.ParseIP(hop)
   if ipAddr == nil {
   	return false
   }
   (none)
   ```
