## 1. Config

- [ ] 1.1 Add `IPHeaderStrategyRightmostUntrusted` and `Config.TrustedProxies`. Accept the strategy in `Prepare`. Reject `remoteAddress` when it is listed and not last. Reject non-empty `trustedProxies` with any other strategy. Other strategies: no new validation
- [ ] 1.2 `NewCore`: build the trusted-proxy helper with the shared builder (plain address → `/32` or `/128`, then `AddCIDR`); store it on `Plugin`. Invalid entry fails creation and names the entry. Empty list stays valid. Missing `remoteAddress` with this strategy: one WARN, not a reject. Do not call `loadIPBlockHelper`

## 2. Hop collection

- [ ] 2.1 Split `GetRemoteIPs`: `collectHops(req, names, allLines)` (split, `cleanIPAddress`, no dedup; `Values` vs `Get`) and `dedupeFirst`. `GetRemoteIPs` = `dedupeFirst(collectHops(req, p.ipHeaders, false))`
- [ ] 2.2 Add `rightmostChain`: first forwarding header with hops (`collectHops` allLines true), then append `cleanIPAddress(req.RemoteAddr)` when `remoteAddress` is listed. Do not concatenate later forwarding headers
- [ ] 2.3 Add `rightmostUntrusted(hops)`: walk from the right, skip `Contains` hits, evaluate exactly one hop. `Contains` error → not trusted. Unparsable hop stays the evaluated hop. All trusted → empty slice (existing empty-hop / `banIfError`). Do not call `GetRemoteIPs`

## 3. Request path

- [ ] 3.1 `ServeHTTP`: for `RightmostUntrusted` only, `hops := rightmostChain(req)`, `remoteIPs := rightmostUntrusted(hops)`, `ipChain := traceIPChain(hops)`. Other strategies keep `GetRemoteIPs`. Leave `skipIP` unchanged
- [ ] 3.2 Trace/error: selected hop on `ip` or `selected_ip`; all-trusted DEBUG with the chain; pass the full chain into `logLookupError`

## 4. Tests

- [ ] 4.1 `zzz_plugin_rightmost_untrusted_test.go`: T1 dedup bypass, T2–T3 multi-line, T4–T6c alternatives / `remoteAddress` / untrusted peer, T7–T10 `trustedProxies` load, T11 all-trusted `banIfError`, T12 trace attributes
- [ ] 4.2 Confirm `zzz_plugin_ipheaders_test.go` `Deduplication_Preserves_First_Occurrence` and `RemoteAddress_Deduplication` still pass (T13). `go test ./...`

## 5. Docs

- [ ] 5.1 README Client IP: alternatives + first present, `remoteAddress` last, residual strip duty, Traefik untrusted-peer XFF-empty / `X-Real-Ip` filled (from `knowledge/research/ext_traefik_forwardedheaders_trustedips/`), `trustedProxies` rejected with other strategies. Do not name a proxy product for multi-line XFF. Keep the Traefik `excludedIPs` claim only after 2.x–3.x
