---
url: https://github.com/maxmind/geoipupdate/blob/main/doc/docker.md
title: geoipupdate Docker
fetched: 2026-08-27
authority: official
ref: github.com/maxmind/geoipupdate:doc/docker.md (main)
---

Image: `ghcr.io/maxmind/geoipupdate`. Required env: `GEOIPUPDATE_EDITION_IDS` (space-separated), plus account ID and license key (or `*_FILE` variants).

Official docker-compose example sets:

```
GEOIPUPDATE_EDITION_IDS=GeoLite2-ASN GeoLite2-City GeoLite2-Country
```

Default database directory in the container: `/usr/share/GeoIP`.
