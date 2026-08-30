## 1. Title

- [x] 1.1 README H1 is Traefik Geoblock and Geoenrich.

## 2. Integration harness

- [x] 2.1 Compose `/enrichthenblock`: `geoblock-chain-enrich` (`mode` `enrich`) then `geoblock-chain-block` (`mode` `block`), shared `countryHeader`.
- [x] 2.2 Compose `/blockonly`: `geoblock-blockonly` (`mode` `block`, `banIfError` true), no lookup source.
- [x] 2.3 Pester: US blocked and DE allowed on `/enrichthenblock`; DE response shows `X-Ipcountry`.
- [x] 2.4 Pester: public IP with no country header on `/blockonly` is 403.

## 3. Spec and usage

- [x] 3.1 Live spec `core_geoblock_plugin_request-mode`: enrich-then-block hop scenario.
- [x] 3.2 Test-harness packet: document `/enrichthenblock` and `/blockonly`.
