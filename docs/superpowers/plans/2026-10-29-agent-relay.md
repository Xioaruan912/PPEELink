# Agent Relay Implementation Plan
> For agentic workers: use superpowers:subagent-driven-development for the independent engine and gateway components; root integrates API/UI and performs final review.
Goal: ship native Agent port forwarding and stable gateway routing with dedicated navigation.
Architecture: relaycore owns native TCP/UDP; xraycore Agent applies RelaySpec separately from Xray; models/API owns rules and generated nodes. Gateways reuse ProxyInbound with secure Reality/HY2 EgressSpec.
Spec: ../specs/2026-10-29-agent-relay.md
Global constraints: keep existing routes; final user revision removes client-side chains; feature capability 5; no arbitrary production rules; reject private targets, cycles and port conflicts; keep bounded resource usage.
Review focus: UDP session affinity; old Agent silently ignoring rules; generated links retaining SNI/pins; stable gateway keys after edit; bad new listener must retain healthy rules.
- [x] Task 1: relaycore Spec/Manager/Status + xraycore Agent integration and bounded TCP/UDP tests.
- [x] Task 2: secure HY2 EgressSpec, API node conversion/options, capability gate, real Xray gateway tests.
- [x] Task 3: models RelayRule + admin API CRUD/runtime integration/traffic deltas + generated nodes and diagnosis mapping tests.
- [x] Task 4: dedicated relay page using existing InboundDrawer for gateways and pure forwarding editor; backend route permissions and navigation tests. Final user revision removes the client-chain section and associated client-policy generation.
- [x] Task 5a: full Go tests with real kernel, API/relay/Xray race tests, Vue checks/build, desktop/390px/320px browser validation and independent engine/gateway review.
- [x] Task 5b: panel deployment, Agent updates and live verification. Current continuation requests completion/deployment; source commit/push is not part of this continuation.
Ruling: reuse the clean existing /root/SubL checkout and explicitly separate worker-owned files; user previously authorized work and pushes on this shared main checkout. No unrelated work is present.

Continuation verification (2026-10-29): full real-kernel Go suite passed with XRAY_FEATURE_BINARY and XRAY_WEBSITE_E2E_BIN pointing to official Xray 26.3.27; race checks for api/relaycore/xraycore/middlewares passed; 31 frontend regressions, vue-tsc and production Vite build passed. Browser verified all three modes, validation, fixed gateway identity and no overflow at 1280/390/320px. Fixed half-close/one-way TCP activity, UDP listener/session replacement, last-good recovery/removals, bounded error history, strict gateway parameter rejection, Agent-side public DNS pinning, and server deletion cleanup. Build temporary directory moved to disk because /tmp tmpfs is full.

Final revisions: only two modes remain; generated native/gateway names exactly match user input. Fixed node visibility after Agent confirmation and cached-page reactivation, and same-remark direct/gateway omission. Full Go suite, API/models race tests, Vue typecheck/build, two-mode 1280/390/320px browser checks and node activation/group/polling/deactivation checks passed.

Final panel: 2026.10.29-relay.85eea5dbb6, SHA-256 10a91bcb10bd4a2aae26a252633abed98b4ee11f17af97c8fcae471d17ba5db0. All five Agents confirmed capability 5, four enabled proxies running, WEB proxy disabled. Production verified frontend asset hashes, authentication/menu, database integrity, existing record IDs/inbounds, user-created native nodes 269/270, recovered gateway node 273, and exact user-entered names. Both native lines and the gateway passed tunnel/HTTPS requests; native traffic counters reported. Active nodes contain no client-chain flags.
