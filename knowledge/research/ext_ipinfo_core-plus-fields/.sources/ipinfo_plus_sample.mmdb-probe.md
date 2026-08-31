---
url: file:ipinfo_plus_sample.mmdb
title: IPinfo Plus sample MMDB decode (oschwald v1)
fetched: 2026-08-31
authority: source
ref: github.com/oschwald/maxminddb-golang@v1.13.1 against ipinfo/sample-database@ff663e00 IPinfo Plus/ipinfo_plus_sample.mmdb
---

Metadata: DatabaseType `ipinfo bundle_location_plus_sample.mmdb`; IPVersion 6; NodeCount 265; RecordSize 32; BuildEpoch 1787480579.

`1.0.0.1` decoded as a flat map: Core keys plus `as_changed=2021-05-01`, `geo_changed=2026-02-08`, `geoname_id=2147714`, `radius=5000`, `is_proxy=false`, `is_relay=false`, `is_tor=false`, `is_vpn=false`. Empty sample fields (`dma_code`, `carrier_name`, `mcc`, `mnc`, `privacy_name`) omitted from the map.

`1.0.4.1`: `country_code=AU`, `region=Victoria`, `region_code=VIC`, `city=Melbourne`, `asn=AS38803`, `as_name=Gtelecom Pty Ltd`, `as_domain=gtelecom.com.au`, `as_type=isp`.

No `network` key. No `isp` key. Not a GeoIP2 nest.
