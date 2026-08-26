# IP2Location download database codes

The official token download API takes a **package / download code** in `file=`. It does not take a ZIP filename.

## Token download URL

Official GET form ([IP2Location Download Client](https://www.ip2location.com/free/downloader)):

```
https://www.ip2location.com/download?token={DOWNLOAD_TOKEN}&file={DATABASE_CODE}
```

`token` is the account download token from the IP2Location Download page. Do not store a real token in this folder.

The documented `curl` invocation uses `-L` (follow redirects). From May 2025, downloads use R2 presigned URLs and redirect over HTTPS to `ip2location.5d193fbd9b158504f07026ff6808d20f.r2.cloudflarestorage.com`. The same page says the HTTP client must follow redirects and must not block the redirect host.

Extract: [.sources/ip2location-download-client.md](.sources/ip2location-download-client.md)

## `file=` is a package code, not a ZIP name

On that page, `{DATABASE_CODE}` is the **database package** (also called package code or download code). The ZIP name is a **local output** argument (`-O` / `-o {LOCAL_ZIP_FILE_NAME}`), not the `file=` value.

Do not pass archive names such as `IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP` as `file=`. Those are filenames, not package codes.

Owner: [IP2Location Download Client](https://www.ip2location.com/free/downloader) documents `file={DATABASE_CODE}`. The official PHP BIN downloader sends the same `file` field as the package code (`dbCode`) to `https://www.ip2location.com/download` ([IP2Location-PHP-Module@188f7362:ip2location_bin_download.php](https://github.com/ip2location/IP2Location-PHP-Module/blob/188f7362c4efd17db218e91b65a18fd4f74ba3bb/ip2location_bin_download.php)). That script uses POST; the documented wget/cURL clients use GET with the same query names.

Extracts: [.sources/ip2location-download-client.md](.sources/ip2location-download-client.md), [.sources/ip2location_bin_download.php.md](.sources/ip2location_bin_download.php.md)

## Paid commercial IPv6 BIN codes

The official Download Client package list includes `DB1BINIPV6` … `DB26BINIPV6` (commercial IPv6 BIN). The paid **DB8 IPv6 BIN** code is therefore **`DB8BINIPV6`**.

Same family on the official PHP README: `DB1BINIPV6` … `DB26BINIPV6`.

IPv4 BIN commercial codes are `DB1BIN` … `DB26BIN`. CSV-style packages on the Download Client page are `DB1` … `DB26` and `DB1IPV6` … `DB26IPV6`. IP2Proxy codes (`PX1` … `PX12`, `PX1BIN` … `PX12BIN`) are listed there too; they are not IP2Location BIN.

Owner: [IP2Location Download Client](https://www.ip2location.com/free/downloader). PHP README repeats the BIN subset.

Extracts: [.sources/ip2location-download-client.md](.sources/ip2location-download-client.md), [.sources/IP2Location-PHP-Module-README.md](.sources/IP2Location-PHP-Module-README.md)

## LITE IPv6 BIN codes

LITE package codes are **not** on the Download Client page (that page lists commercial IP2Location and IP2Proxy only). The official PHP module README owns the LITE list:

- IPv6 BIN: `DB1LITEBINIPV6` … `DB11LITEBINIPV6`
- IPv4 BIN: `DB1LITEBIN` … `DB11LITEBIN`

Owner: [IP2Location-PHP-Module README, BIN DOWNLOADER SCRIPT](https://github.com/ip2location/IP2Location-PHP-Module/blob/188f7362c4efd17db218e91b65a18fd4f74ba3bb/README.md) (`repo@188f7362c4efd17db218e91b65a18fd4f74ba3bb:README.md`).

Extract: [.sources/IP2Location-PHP-Module-README.md](.sources/IP2Location-PHP-Module-README.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| GET URL `token` + `file` | https://www.ip2location.com/free/downloader | official |
| `file=` is `{DATABASE_CODE}` / package code | same | official |
| Official `curl` uses `-L`; follow R2 redirects | same | official |
| Paid IPv6 BIN codes `DB1BINIPV6`…`DB26BINIPV6` (so DB8 = `DB8BINIPV6`) | same | official |
| LITE IPv6 BIN codes `DB1LITEBINIPV6`…`DB11LITEBINIPV6` | IP2Location-PHP-Module README | official (vendor README) |
| Downloader script posts `file` = package code | IP2Location-PHP-Module@188f7362:ip2location_bin_download.php | source |

No conflict on commercial IPv6 BIN codes. LITE codes appear only on the official PHP README, not on the Download Client page.

## References

- https://www.ip2location.com/free/downloader
- https://github.com/ip2location/IP2Location-PHP-Module/blob/188f7362c4efd17db218e91b65a18fd4f74ba3bb/README.md
- https://github.com/ip2location/IP2Location-PHP-Module/blob/188f7362c4efd17db218e91b65a18fd4f74ba3bb/ip2location_bin_download.php
