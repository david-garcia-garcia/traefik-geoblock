## 1. Search glob

- [ ] 1.1 `fileutils.Search`: exact directory skips glob; env glob is exclusive (zero directory matches do not fall through); unset tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Skip non-directory Glob hits. Do not derive `GoPath()` anywhere else.
- [ ] 1.2 Several matches: first sorted Glob match containing `seeds/<that call's defaultFile>`, else `<defaultFile>` at that directory. None → not-found. Ban HTML still uses `geoblockban.html` at the directory.
- [ ] 1.3 Miss logs: Search MUST NOT Error. File miss DEBUG. Exclusive glob/directory miss WARN. `chooseStartupTarget` DEBUG when Latest already won and BundledFile misses. `initialize` WARN when waiting with no file.
- [ ] 1.4 Native `pkg/fileutils` tests: env glob with a temp `gop-*` layout; exact directory skips glob; env glob zero matches does not fall through; several matches prefer `seeds/<defaultFile>`. Update the unset-env and env-is-a-file cases. No Traefik-container glob test.

## 2. Wait for URL

- [ ] 2.1 `openCatalogRow`: `AllowMissing` when URL is non-empty or (`Path=="" && DefaultFileName==""`). `MMDBConfig` gains `AllowMissing`; `newMMDB` uses it. Missing `defaultFile` with no URL still fails.
- [ ] 2.2 `initialize`: WARN `no database file yet; waiting for auto-update` and return; `startUpdate` still runs. Nil-handle errors stay `BIN is not open` / `MMDB is not open`.
- [ ] 2.3 Tests: BIN and MMDB URL + missing `defaultFile` start with WARN; missing `defaultFile` and no URL still fails New. No `zzz_proof_*`.

## 3. Docs

- [ ] 3.1 README Local plugin: Traefik `gop-*` under `/plugins-storage/sources` (`{dir}/sources/gop-<random>/src/github.com/david-garcia-garcia/traefik-geoblock`). Keep the `*` glob. Exact `/plugins-local/...` stays valid. Document `banIfError: false` as fail-open on lookup only (CIDR / private / country / `defaultAllow` still apply). Default stays true.
- [ ] 3.2 Usage packet `knowledge/devdocs/core_geoblock_database_source.md` gotcha: glob accepted; unset tries the default globs; env glob does not fall through; miss levels. Do not document a second plugin-directory calculation.
