# Delivery

## Motivation
Operators who put this plugin behind one or more reverse proxies need one evaluated hop: the original client, not the load balancer. Dest only offers `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`, which concatenate every present `ipHeaders` entry, read the first field line, and drop repeated IPs while keeping the first occurrence.

That collection lets a client pick the country. Repeating `8.8.8.8` on both sides of a forged `1.1.1.1` in `X-Forwarded-For` hides the real rightmost untrusted hop after first-occurrence dedup. A second `X-Forwarded-For` field line is invisible to `Header.Get`. Two configured headers on one request are walked as one concatenated list, so `X-Real-IP` can win over `X-Forwarded-For`.

Left alone, a `RightmostUntrusted` strategy that reused `GetRemoteIPs` would ship the Traefik `excludedIPs` shape without the guarantee a client cannot choose the country. Dest has no such strategy yet; CheckAll is a different contract (every hop, including proxies).

Priority: P2 — real operator pain selecting the client hop behind proxies, with CheckAll as a different-semantics workaround

## Implementation
Hop collection is split: `collectHops` reads headers and `cleanIPAddress` with no dedup; `GetRemoteIPs` stays `dedupeFirst(collectHops(..., false))` for the three existing strategies. `RightmostUntrusted` uses `clientHops` → `rightmostChain` (first forwarding header with hops, every field line, `remoteAddress` last when listed) then `rightmostUntrusted` (walk from the right, skip `trustedProxies` on a dedicated `iplookup.Helper`). Empty result reuses the existing empty-hop / `banIfError` path. `Prepare` rejects `trustedProxies` with any other strategy and requires `remoteAddress` last when listed.

## What this changes
**Operators.** Opt-in `ipHeaderStrategy: RightmostUntrusted` and `trustedProxies`. Pairing `trustedProxies` with CheckAll / CheckFirst / CheckFirstNonePrivate fails creation. Default `ipHeaders` stay valid; put `remoteAddress` last so a direct untrusted peer ignores forged headers. Each proxy must overwrite or strip configured headers it does not set.
**Admin users.** None.
**Developers.** `Config.TrustedProxies` and `IPHeaderStrategyRightmostUntrusted` are public. `GetRemoteIPs` remains first-line Get plus first-occurrence dedup; the new strategy must not call it.
**End users.** When operators enable this strategy, geo allow/block follows the rightmost untrusted hop, not a forged left-side IP.
