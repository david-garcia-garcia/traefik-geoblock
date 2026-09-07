## MODIFIED Requirements

### Requirement: BIN Lookup applies mapped Get_all columns
`BIN.LookupRecord` SHALL call `Get_all` once and SHALL copy only mapped paths onto the Record. Unused Get_all columns SHALL not be written. Path `asn` SHALL use the Get_all Asn field. Lookup MUST NOT call `Get_asn`. Path `country_short` SHALL map IP2Location `-`, empty, and unavailable strings to empty on the Record through the same empty-vendor mapping as the other BIN columns. An invalid `country_short` SHALL remain a lookup error. The BIN wrapper MUST NOT write `XX`.

#### Scenario: ASN-only map does not write country
- **WHEN** the Field map is `ip2location_asn`
- **THEN** Lookup does not set country
- **AND** Record ASN is the Get_all Asn value when the file has that column

#### Scenario: BIN country miss is empty
- **WHEN** the Field map includes `country_short` and `Get_all` `country_short` is `-`
- **THEN** Record country is empty
- **AND** Combined MAY copy country from a later source
