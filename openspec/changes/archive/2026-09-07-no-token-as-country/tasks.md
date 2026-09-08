## 1. No token as country

- [x] 1.1 Return a record with no country from both `recordForLookup` error paths
- [x] 1.2 Assert an unparseable IP header value yields `XX`, not the token
- [x] 1.3 Assert a later hop that resolves still wins over an unparseable earlier hop
- [x] 1.4 Assert a resolvable hop and a private hop are unaffected
- [x] 1.5 Assert a lookup error carries no country

## 2. Docs

- [x] 2.1 README: an unparseable IP header value enriches as `XX`
- [x] 2.2 Update `knowledge/devdocs/core_geoblock_plugin_request-mode.md`
