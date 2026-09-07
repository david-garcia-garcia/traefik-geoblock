## MODIFIED Requirements

### Requirement: BIN Lookup applies mapped Get_all columns
`BIN.LookupRecord` SHALL call `Get_all` once and SHALL copy only mapped paths onto the Record. Unused Get_all columns SHALL not be written. Path `asn` SHALL use the Get_all Asn field. Lookup MUST NOT call `Get_asn`. Path `country_short` SHALL be copied through the same empty-vendor mapping as the other BIN columns: IP2Location `-`, empty, unavailable, and invalid strings SHALL become empty on the Record. The BIN wrapper MUST NOT write `XX`.

#### Scenario: ASN-only map does not write country
- **WHEN** the Field map is `ip2location_asn`
- **THEN** Lookup does not set country
- **AND** Record ASN is the Get_all Asn value when the file has that column

#### Scenario: BIN country miss is empty
- **WHEN** the Field map includes `country_short` and `Get_all` `country_short` is `-`
- **THEN** Record country is empty
- **AND** Combined MAY copy country from a later source
