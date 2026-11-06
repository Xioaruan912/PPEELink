# Standalone Agent branch and panel separation

User requests Agent code be independent in the `agent` branch, not mixed with panel development. Preserve Agent1.17 behavior and existing installation, monitor alias, upgrade, protocol, throughput, relay and persistent-terminal compatibility.

- [x] Complete all7 Agent1.17 throughput-tool updates and preserve each server's enabled state.
- [x] Create isolated `agent` branch/worktree from current repository history; copy the current Agent runtime dependency closure only. Independent Go module, CLI entry, tests, build script and CI; no frontend, panel API or database packages.
- [x] Produce standalone amd64/arm64 (and supported ARMv7) Agent artifacts with version/checksum metadata; verify existing CLI and wire protocols without changing Agent behavior.
- [x] Remove remote Agent entry/files from panel source and switch install/update downloads and target hashes to separately provisioned Agent artifacts. Keep panel-side protocol handling and local proxy runtime.
- [x] Verify Agent build/dependency boundary, compatibility-focused tests and panel build; run independent review and deploy coherent artifact/loader update if needed. Preserve rollback backup and active sessions.
- [x] Commit/push standalone Agent source to `agent`; document future independent build/release workflow and final branch URLs.

Branch cleanup requested by user:
- redesign/signal-atlas: tip dcdebb2be70c6a30b86b5067b5b94ed7a9e8ee29 is fully contained in origin/main (74 commits behind, zero unique); no worktree. Remote deleted with exact-head force-with-lease on2026-11-06.
- redesign/console-jade: clean but attached to /root/dev/SubL and contains3 commits absent from main/agents. Preserve; deletion condition is not met.

Current source worktree has many earlier deployed changes; do not reset, discard or broadly stage them. Agent branch snapshot copies only runtime-owned files; panel integration changes remain reviewable in deploy-source.

Checkpoint: all7 updates complete; standalone Agent worktree ../agent-source created on branchagent, runtime-only source copied, module tidy/build in progress. Original monolithic panel worktree untouched by structural deletions yet.

Implementation decisions:
- Standalone module path is github.com/Xioaruan912/PPEELink at branchagent (supports Go install @agent). Module tidy reduces runtime dependencies to pty, websocket, x/net, grpc and protobuf; no Gin/Gorm/SQLite/sing-box. First native standalone build is14MiB vs61MiB mixed program.
- Agent root now holds runAgent/removeAgent in agent_runtime.go; main.go supports version, agent, monitor and probe only. Runtime packages copied without semantic changes; no panel API/model/frontend in tree.
- Standalone build.sh builds three Linux architectures and generates dist/release.json schema: {version:"1.17",artifacts:{"linux/amd64":{file:"agent-linux-amd64",sha256:"..."},"linux/arm64":...,"linux/arm":{file:"agent-linux-armv7",sha256:"..."}}}. CI uploads artifact only, no public release.
- Panel loader will read downloads/agent/release.json (optional PPEELINK_AGENT_RELEASE_DIR override), validate version, architecture, path, regular file and hash; no fallback to panel executable. Existing /agent/binary, monitor/binary and probe/binary URLs remain stable. Spec/ack hashes and displayed target version come from release, not running panel. Unknown initial arch falls back to panel arch; reported Arch stored on ProxyServer; add arch query from new Agent spec requests.
- Port existing root agent_terminal_http_test.go to panel integration package with external Agent test helper subprocess (PPEELINK_AGENT_TEST_BINARY). Helper exists only in Agent test binary, uses explicit test socket/unit and runAgentTerminal; no panel compile dependency on Agent code. Unit/persistent tests move to agent branch; real systemd tests opt in through PPEELINK_AGENT_SYSTEMD_TESTS=1.
- Finish tests and build before panel structural deletes/deployment; artifact provisioning and panel replacement are one coherent cutover. Existing Agent1.17 wire behavior remains; final independent binaries still version1.17 (no functional release change in extraction).

Checkpoint: all7 Agent1.17 upgrades and static tools completed; standalone source/module/build/CI created, panel worker files removed, release provider implemented. Cross-process terminal HTTP authorization/recovery passed using Agent test executable; Agent normal packages and hash/update/stats tests passed. Independent review found default Docker VOLUME would mask newer artifacts; removed it. Original console-jade remains untouched, signal-atlas remote deletion verified. Remaining: final Agent commit/build/push, panel build + artifact staging/deployment, ensure existing17 Agent hashes move to standalone artifact, clean validation credentials and record results.

Checkpoint2: branchagent committed at d036358 and pushed to GitHub (canonical repo renamed from SubL to PPEELink). Runtime-only build14MiB, three architecture release artifacts and manifest built. Agent default modules pass; systemd restart task persistence and cross-process panel terminal HTTP authorization/recovery pass. Panel artifact paths/checksums/ack architecture tests pass. Panel worker root files removed and main builds. Docker CI independently checks out agent and excludes its source from panel build context; default VOLUME regression fixed. Remaining final coherent deployment/artifact staging + optionally move existing1.17 processes to standalone binary; persist final branch/deployment audit and delete validation credentials.

Final source ownership cleanup: panel throughputcore reduced to wire DTOs only; terminalcore runtime removed entirely from panel. External Agent test helper prepares its own isolated tmux manager, leaving panel integration tests with protocol fixture data only. xraycore/monitorcore/relaycore/tracecore retained because panel local proxy/tracing paths use those runtimes. Re-run cross-process proof before final panel rebuild.

Completed delivery: all7 running processes match standalone amd64 artifact SHA9533f38ad07c1423764817b7e1dff7bcc671439348615243c34c71aafe384b1e; all are1.17 with authenticated iperf3 3.22 ready and prior enabled states preserved. Live DMIT canary retained the same task PID/progress through binary replacement. Panel final protocol-only ownership build/staging and cross-process tests passed. Agent source branch HEADb746ca8 (runtime artifacts from707ee36); main synchronization explicitly approved by user on2026-11-06. Main source snapshot includes previously deployed panel work so no production changes are discarded.
