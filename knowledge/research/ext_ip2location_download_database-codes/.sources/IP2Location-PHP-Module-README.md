---
url: https://github.com/ip2location/IP2Location-PHP-Module/blob/188f7362c4efd17db218e91b65a18fd4f74ba3bb/README.md
title: IP2Location-PHP-Module README — BIN DOWNLOADER SCRIPT
fetched: 2026-08-26
authority: official
ref: ip2location/IP2Location-PHP-Module@188f7362c4efd17db218e91b65a18fd4f74ba3bb:README.md
---

Official IP2Location.com PHP module README (commit 188f7362c4efd17db218e91b65a18fd4f74ba3bb).

BIN DOWNLOADER SCRIPT:

```
php ip2location_bin_download.php --token DOWNLOAD_TOKEN --file DATABASE_CODE -y
```

`file` is documented as Database package (package code / download code), not a ZIP filename.

Allowed DATABASE_CODE families on this README:

- Commercial IPv4 BIN: DB1BIN...DB26BIN
- Commercial IPv6 BIN: DB1BINIPV6...DB26BINIPV6
- LITE IPv4 BIN: DB1LITEBIN...DB11LITEBIN
- LITE IPv6 BIN: DB1LITEBINIPV6...DB11LITEBINIPV6

Therefore LITE IPv6 BIN codes are DB1LITEBINIPV6 through DB11LITEBINIPV6. Paid DB8 IPv6 BIN is DB8BINIPV6 (within DB1BINIPV6...DB26BINIPV6).

Token and package code are obtained from the IP2Location Account Area Download page.

IPv4 BIN vs IPv6 BIN: IPv4 BIN queries IPv4 only. IPv6 BIN queries both IPv4 and IPv6.
