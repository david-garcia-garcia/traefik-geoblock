# Import reclaim table from traefik-middleware-utilities v1.0.1

Reclaim the custom reclaim table in this project by importing the reclaim table from https://github.com/david-garcia-garcia/traefik-middleware-utilities version v1.0.1.
It is possible that some things are reshaped; the remote component is more flexible and robust — adjust to its shape as needed.
Finished when the PR is passing CI and the delivery card is updated.
The reclaim table we will be using DOES have Sleep, Wake, and Close hooks. Import and wire the v1.0.1 Hooks shape (Sleep, Wake, Close). Do not assume a Close-only local copy is the target. Reshape local callers to the remote Hooks API as needed.
