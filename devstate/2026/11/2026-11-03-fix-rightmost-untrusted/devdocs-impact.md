# Devdocs impact
change: fix-rightmost-untrusted

## Units
- IP header strategy — subsystem — `pkg/geoblock/ip.go` / `openspec/changes/fix-rightmost-untrusted/specs/core_geoblock_plugin_ip-header-strategy`
- Family-isolated CIDR helper — pattern — `pkg/geoblock/ip.go` `newTrustedProxyHelper` / `std_go_iplookup`
- Request mode — subsystem — `pkg/geoblock/plugin.go` empty-hop / `banIfError` (`core_geoblock_plugin_request-mode`)

## Findings
- [x] stale-usage  Request mode — `core_geoblock_plugin_request-mode` How-to still names empty `GetRemoteIPs`; ServeHTTP now uses `clientHops` (empty `GetRemoteIPs` or empty `rightmostUntrusted`)
