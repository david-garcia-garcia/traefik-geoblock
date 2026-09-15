## 1. Official LITE seed

- [ ] 1.1 GET `https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP`, extract `IP2LOCATION-LITE-DB1.IPV6.BIN`, and confirm it opens (`8.8.8.8` → `US`, `1.1.1.1` → `AU`)
- [ ] 1.2 Overwrite `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN` with that extract (do not use `tools/dbdownload` write; verify still requires `US`)
- [ ] 1.3 Leave `seeds/ipinfo_lite.mmdb` and `seeds/GeoIP2-Country-Test.mmdb` unchanged

## 2. Verify

- [ ] 2.1 Run the existing package tests that open the LITE seed (`pkg/dbwrappers`, `pkg/geoblock`)
- [ ] 2.2 Do not change `tools/dbdownload` `verifyDatabase`
