# Get newer versions of the committed seed databases

Get newer versions of the committed seed databases and update them in the repo. Use a dedicated worktree. The shipped seeds today are:

- seeds/IP2LOCATION-LITE-DB1.IPV6.BIN (IP2Location LITE country IPv6 BIN; tools/dbdownload downloads https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP)
- seeds/ipinfo_lite.mmdb (IPinfo Lite snapshot)
- seeds/GeoIP2-Country-Test.mmdb (MaxMind official dummy Country fixture)

Replace those committed files with newer official builds of the same product/edition. Do not add a new vendor, new public config, or new seed filenames unless the vendor renamed the same product. Do not change plugin behavior except as required for the newer files to open.
