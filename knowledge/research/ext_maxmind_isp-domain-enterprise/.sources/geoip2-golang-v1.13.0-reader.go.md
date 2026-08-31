---
url: https://github.com/oschwald/geoip2-golang/blob/v1.13.0/reader.go
title: geoip2-golang v1.13.0 reader.go (ISP / Domain / Enterprise)
fetched: 2026-08-31
authority: source
ref: github.com/oschwald/geoip2-golang@v1.13.0:reader.go
---

Package comment: structs match the internal MaxMind database structure. Unofficial (not a MaxMind API). Built on `github.com/oschwald/maxminddb-golang`.

`ISP` (flat): `autonomous_system_organization`, `isp`, `mobile_country_code`, `mobile_network_code`, `organization`, `autonomous_system_number`. `DatabaseType` `GeoIP2-ISP`, `GeoIP2-Precision-ISP`.

`Domain` (flat): `domain`. `DatabaseType` `GeoIP2-Domain`.

`Enterprise` nest: `continent` (`names`, `code`, `geoname_id`), `city` (`names`, `geoname_id`, `confidence`), `subdivisions[]` (`names`, `iso_code`, `geoname_id`, `confidence`), `country` (`names`, `iso_code`, `geoname_id`, `confidence`, `is_in_european_union`), `traits` (`autonomous_system_organization`, `connection_type`, `domain`, `isp`, `mobile_country_code`, `mobile_network_code`, `organization`, `user_type`, `autonomous_system_number`, plus unofficial extras `static_ip_score`, `is_anonymous_proxy`, `is_legitimate_proxy`, `is_satellite_provider`). `DatabaseType` `GeoIP2-Enterprise`.

`reader_test.go` (same commit): Domain `1.2.0.0` → `maxmind.com`. Enterprise `149.101.100.0` → `traits.isp=Verizon Wireless`, `traits.autonomous_system_number=6167`. ISP same IP → `ISP=Verizon Wireless`, `AutonomousSystemNumber=6167`.
