# Specs
change: cidr-family-isolation

- added core_geoblock_iplookup_family-match
  verdict: new
  confidence: high
  candidates: core_geoblock_plugin_request-mode, core_geoblock_iplookup_family-match
  why: request-mode owns hop/mode CIDR application, not family isolation of the radix. No existing `core_geoblock_iplookup_*` leaf. Usage packet is `core_geoblock_iplookup.md`. 4th part names the match contract, not the change kebab.
