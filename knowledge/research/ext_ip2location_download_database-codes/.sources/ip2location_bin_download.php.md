---
url: https://github.com/ip2location/IP2Location-PHP-Module/blob/188f7362c4efd17db218e91b65a18fd4f74ba3bb/ip2location_bin_download.php
title: ip2location_bin_download.php
fetched: 2026-08-26
authority: source
ref: ip2location/IP2Location-PHP-Module@188f7362c4efd17db218e91b65a18fd4f74ba3bb:ip2location_bin_download.php
---

Official BIN downloader in IP2Location-PHP-Module at commit 188f7362c4efd17db218e91b65a18fd4f74ba3bb.

Download call posts the package code as `file` (not a ZIP filename) to the same path the docs document for GET:

```
$queries = [
  'token' => $token,
  'file' => $dbCode,
];
$response = post('https://www.ip2location.com/download', $queries, $fileName);
```

`$fileName` is the local ZIP path written by cURL. `$dbCode` is the DATABASE_CODE / package code.

A prior POST to `https://www.ip2location.com/download-info` uses `token` and `package` => `$dbCode` to verify the account and obtain expected file size. Responses include OK, EXPIRED, NOPERMISSION.

The helper uses `CURLOPT_POST` (POST body), unlike the documented wget/cURL GET with query string. Same field names: `token`, `file`.
