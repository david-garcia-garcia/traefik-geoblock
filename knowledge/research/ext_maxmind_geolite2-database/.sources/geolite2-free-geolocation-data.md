---
url: https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
title: GeoLite Databases and Web Services
fetched: 2026-08-27
authority: official
---

Free geolocation and ASN data as downloadable databases and web services. Signup: https://www.maxmind.com/en/geolite2/signup

License key authenticates database downloads and web service requests. Generate keys after creating a MaxMind account.

Formats: binary `.mmdb` (fast lookups) and CSV. GeoLite users limited to 30 database downloads per day. GeoLite EULA requires keeping data up to date: delete GeoLite databases within 30 days of a new release.

Three GeoLite databases:

- GeoLite Country — country-level geo for analytics, content customization, or compliance in territories that are not disputed. Some fields on the shared City/Country docs are not in GeoLite; check field descriptions.
- GeoLite City — city/postal; considerably less accurate than paid GeoIP City; not recommended for commercial use cases.
- GeoLite ASN — ASN and organization for analytics.

Lookup binary DBs with MaxMind client APIs (same methods as GeoIP).
