---
url: https://www.ip2location.com/free/downloader
title: IP2Location Download Client
fetched: 2026-08-26
authority: official
---

Starting in May 2025, all database downloads utilize R2 presigned URLs. HTTP clients must follow redirects. Proxy or firewall must not block the redirect host.

Redirect hostname (HTTPS): ip2location.5d193fbd9b158504f07026ff6808d20f.r2.cloudflarestorage.com

Token download URL (wget):

```
wget "https://www.ip2location.com/download?token={DOWNLOAD_TOKEN}&file={DATABASE_CODE}" -O {LOCAL_ZIP_FILE_NAME}
```

Token download URL (cURL; official example uses -L):

```
curl -L -o {LOCAL_ZIP_FILE_NAME} "https://www.ip2location.com/download?token={DOWNLOAD_TOKEN}&file={DATABASE_CODE}"
```

`file` is `{DATABASE_CODE}`, the database package. The ZIP name is `{LOCAL_ZIP_FILE_NAME}` (local output), not the `file=` value.

Download Client `package` parameter (same codes as `file=` / DATABASE_CODE):

- DB1...DB26
- DB1BIN...DB26BIN
- DB1IPV6...DB26IPV6
- DB1BINIPV6...DB26BINIPV6
- PX1...PX12
- PX1BIN...PX12BIN

The page calls these the package code or download code. Account Download page is where subscribers copy the code and token.

Paid commercial IPv6 BIN therefore uses DB1BINIPV6 through DB26BINIPV6. DB8 IPv6 BIN is DB8BINIPV6.

This page does not list LITE package codes (no *LITEBIN* family).

IP2Location databases update on the first day of the calendar month. IP2Proxy databases update daily (download after 00:15 GMT).
