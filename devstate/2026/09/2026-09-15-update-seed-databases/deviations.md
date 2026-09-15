# Deviations

- [x] taken  skip IPinfo and MaxMind seed bytes; refresh IP2Location LITE only
  Asked: replace the three committed seeds with newer official builds of the same product/edition.
  Instead: write only `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN` from the official LITE ZIP. Leave `seeds/ipinfo_lite.mmdb` and `seeds/GeoIP2-Country-Test.mmdb` unchanged.
  Owner: `seeds/ipinfo_lite.mmdb`
  Why: official full IPinfo Lite is token-gated and this environment has no token; the 100-row sample is a different edition. The MaxMind dummy matches `maxmind/MaxMind-DB` `test-data` byte-for-byte.
  By: explore
  Requester: not asked
