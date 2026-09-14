# CIDR rules must match only their address family

Fix F-2 from geoblock-bug-hunt-2026-09-14.md.

F-2: pkg/iplookup insert/contains put IPv4 and IPv6 CIDRs in one radix path. IPv4 prefixes are walked from bit 96 of the mapped form but inserted from the root, where IPv6 prefixes start. An IPv4 a.b.c.d/32 also matches IPv6 whose first four bytes are a b c d; an IPv6 /32 matches the IPv4 made of its first four bytes. Operator allow-lists an IPv6 office prefix and silently allow-lists unrelated IPv4 (pass:allowed_ip_block on a country-blocked client). Mirror case blocks innocent traffic.

Files: pkg/iplookup/iplookup.go insert/contains; consumed by pkg/geoblock/plugin.go decide.

Existing mixed-family test only uses non-colliding prefixes (192.168.1.0/24 vs 2001:db8::/32).

Desired: a CIDR rule matches only the address family it was written for. Keep longest-prefix behaviour within a family. Consume MMDB/BIN-style existing patterns; prefer two trees or a family discriminator on the endpoint — shape the ask to the system, smallest durable delta.

Out of scope: F-1 BIN race, F-3 bypassHeaders, F-4 updater Stop, F-5 /0 sentinel (related tree API but not this ticket unless your fix to found/length is required to keep current /0 tests passing — do not take F-5's documented longest-prefix vs /0 product change unless explore shows it is the same owner and inseparable). Do not copy zzz_proof_* filenames; write proper product tests that cover the colliding prefixes from the report (1.2.3.4/32 vs 102:304::1; 808:808::/32 vs 8.8.8.8; request-path IPv6 allow vs blocked-country IPv4).
