# Standalone Agent release input

Agent source is maintained on the `agent` branch, with its own Go module and build pipeline.
For a panel image containing Agent downloads, copy the complete `dist/` from that branch
here before `docker build`. The files are build inputs, not panel source, and are ignored
by Git. CI checks out the independent branch outside the panel build context and supplies
the release artifacts automatically.

For a running non-Docker panel, publish the same files under `downloads/agent/` in the
panel working directory (or `PPEELINK_AGENT_RELEASE_DIR`). Upload binaries before atomically
activating `release.json`. Missing or invalid artifacts produce an explicit503; the panel
never serves its own executable as an Agent.
