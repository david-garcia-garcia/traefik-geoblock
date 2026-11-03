# Spec

1. [wrong] Only-path bundled miss is DEBUG — `pkg/fileutils/fileutils.go:187` — `Download component resolves the file location` SHALL WARN a bundled-file miss that is the only remaining path, and the env-set exact-files-missing scenario SHALL WARN those tried paths and that the env is probably not the plugin directory; `bundledFileUnder` logs DEBUG. A dated-won miss SHALL DEBUG; when the env is unset, `Search` can still WARN that the env must be set after Latest already won.
   Fix: WARN the tried `seeds/` and root paths and that the env is probably not the plugin directory when the bundled miss is the only remaining Resolve path; DEBUG that miss only when a dated catalog file already won, with no extra WARN
   Status: done
   Argument: bundledFileUnder WARNs the tried paths; dated-won seedFirst peeks BundledFile with discarded logs and DEBUGs.
