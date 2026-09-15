# Replace local pkg/iplookup with traefik-middleware-utilities v1.0.2

Replace the in-repo pkg/iplookup package with an import of the iplookup package from github.com/david-garcia-garcia/traefik-middleware-utilities at release v1.0.2 (https://github.com/david-garcia-garcia/traefik-middleware-utilities/releases/tag/v1.0.2). That release adds a family-isolated CIDR helper (IPv4 and IPv6 on separate trees) so a prefix cannot match the other family.

Do not land or continue PR 86 (https://github.com/david-garcia-garcia/traefik-geoblock/pull/86) or branch 2026-09-14-cidr-family-leak. That PR is closed. The family-isolation fix ships by consuming the utilities package, not by rewriting the local helper in this repo.

In scope:
- Bump github.com/david-garcia-garcia/traefik-middleware-utilities from the current v1.0.1 (already required in go.mod for reclaim) to v1.0.2.
- Switch all product call sites from github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup to github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup.
- Remove the local pkg/iplookup package unless explore finds a contract mismatch that requires a thin adapter; prefer delete + call-site update.
- Preserve geoblock CIDR allow/block behavior (allowedIPBlocks / blockedIPBlocks and directory lists). The utilities helper already isolates families.
- Vendor as this plugin repo already vendors (Yaegi).
- Update OpenSpec / knowledge/devdocs that name the local helper.

Out of scope:
- Reopening or merging PR 86 / 2026-09-14-cidr-family-leak.
- Importing other unused utilities packages.
- Changing decide /0 sentinel vs longest-prefix policy.
- CI -race overhaul.
- Copying or committing zzz_proof_* files from the caller workspace.
