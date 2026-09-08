## prepare (2026-09-08)
phase: prepare
findings: none
fixed: n/a
skipped: worker delegation — prepare ran on the conductor thread (local issue host; the caller spec was already in context, and the heavy prior-art read was delegated to a background subagent instead)
