# Specs
change: fix-rightmost-untrusted
- added core_geoblock_plugin_ip-header-strategy
  FindSpecHost: new, high
  candidates: core_geoblock_plugin_request-mode, std_go_iplookup_family-isolated, core_geoblock_observability_decision-header, core_geoblock_ip-header-strategy (illegal 3-part)
  skip: request-mode (empty hop / banIfError owner), iplookup (Contains owner), observability (R5 is tasks, not a decision-header delta)
