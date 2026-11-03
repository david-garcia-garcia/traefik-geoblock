# Standards

1. [hard] Leave a trail — `pkg/fileutils/fileutils.go:85` — `PluginPathEnv` is still called a directory after Search accepts a glob
   Quote:
   ```
   // PluginPathEnv is the plugin-root directory (local clone or registry unpack).
   const PluginPathEnv = "TRAEFIK_PLUGIN_GEOBLOCK_PATH"
   ```
   Fix: Document the env as an exact plugin directory or a glob
   Status: done
   Argument: PluginPathEnv comment now says exact directory or glob.
2. [hard] Bound the ask — `knowledge/opendev.json:2` — model routing keys were renamed and empty `model_opd-*` slots added; this change did not need them
   Quote:
   ```
   "model_default": "cursor-grok-4.6-high",
   "model_opd-archive": "",
   ```
   Fix: Drop the opendev.json model-key rewrite from this change
   Status: done
   Argument: Restored knowledge/opendev.json to origin/master (dropped model-key rewrite).
