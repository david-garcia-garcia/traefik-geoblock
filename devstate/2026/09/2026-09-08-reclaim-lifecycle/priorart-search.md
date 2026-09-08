# Prior-art search: does a sleep/wake reclaim already exist?

Answer: no. Designed from scratch, with PR #82 read as prior art for the edge cases.

## What was searched

GitHub code search over `user:david-garcia-garcia`, via the GitHub MCP namespace.

| Query | Hits |
| --- | --- |
| `reclaim Wake Sleep user:david-garcia-garcia language:go` | 0 |
| `"func (t *Table) Open" user:david-garcia-garcia language:go` | 2: `traefik-geoblock/pkg/reclaim/table.go`, `traefik-modsecurity/pkg/reclaim/table.go` |
| `"interface{ Wake()" OR "Sleep()" user:david-garcia-garcia language:go` | 0 |

## Conclusion

The reclaim table exists in exactly two repositories, and they are the two known copies of the
same shared component. `traefik-modsecurity` had already been checked by the human and has no
`Wake` or `Sleep`. There is no third implementation to follow, so the lifecycle in this change is
new work.

The one caveat GitHub code search carries: it indexes default branches, so an implementation
living only on an unmerged branch would not appear. `traefik-modsecurity` was checked directly,
and it is the only other repository holding this component.
