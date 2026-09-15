## 1. Any hop can deny

- [x] 1.1 Drop `passReason == PhaseNone` from the deny in `blockFromHeader`
- [x] 1.2 Drop `passReason == PhaseNone` from the in-loop `banIfError` ban
- [x] 1.3 Drop the unreachable `&& allowed` from the `passReason` assignment
- [x] 1.4 Assert `blockedIPBlocks` denies the last hop and a middle hop after an allowed first hop
- [x] 1.5 Assert `blockedCountries` denies a public hop behind an allowed private hop
- [x] 1.6 Assert an `allowedIPBlocks` hop does not exempt the hops after it
- [x] 1.7 Assert `banIfError` bans an unparseable later hop in `block` mode
- [x] 1.8 Assert a chain no rule matches still passes `pass:default_allow`, and a private-only chain still passes `pass:allow_private`

## 2. Docs

- [x] 2.1 README: behaviour change, and that `allowedIPBlocks` cannot allow a private hop
- [x] 2.2 Update `knowledge/devdocs/core_geoblock_plugin_request-mode.md`
