# Network path and Agent throughput implementation plan

> Use superpowers:executing-plans; independent trace and runner modules use dispatching-parallel-agents.

**Goal:** Compare candidate relay VPS paths and measured bidirectional capacity to one managed landing.
**Architecture:** Keep authenticated panel job orchestration, bounded Agent commands, persistent results. Upgrade traceroute to structured NextTrace paths; temporary authenticated iperf3 listeners on landing and source-side clients transfer directly. Sequential candidates and directions preserve comparability.
**Tech stack:** Go/Gin/GORM, Vue 3/Element Plus, NextTrace, iperf3.
**Spec:** Approved research in this chat, 2026-10-29.

## Global constraints
- Preserve all existing uncommitted relay/delivery changes. Work in place because deployed relay features are not yet committed; no reset/stash or unrelated rewriting.
- Only managed Agents participate in throughput. Never aim iperf at proxy protocol ports.
- Duration 5–30 seconds per direction, streams 1–8, candidates <=8, finite configurable traffic/rate limits. IPv4 and IPv6 explicit.
- Source identity from Agent token only. Lease expiry, cancellation, restart and output limits must terminate both sides. No permanent anonymous listener. Minimum iperf3 3.22 (fixed authentication issues); no weak padding downgrade.
- Defaults: TCP, sequential both directions, 10 seconds, one stream, 100 Mbps aggregate rate. Cap reached displayed as capped, not proven capacity.
- Preserve route URL /network/traceroute; add /network/throughput. Mobile 320px, clear failures, no uncontrolled page overflow.

## Review focus
- Cross-Agent job theft/report tampering and late report after cancellation rejected.
- Landing readiness gates client dispatch; same endpoint cannot participate in overlapping measurements.
- Tool unavailable/version mismatch reported clearly; no silent downgrade to weaker auth.
- Unknown path hops do not merge; missing responses never called business loss.
- Result direction and receive-side throughput correct for reverse and simultaneous bidirectional runs.

## Task 1: Path topology
Files: tracecore/*, api/network_trace.go, models/network_trace.go, agent_trace_client.go, probe_trace_client.go, webs/src/api/network/trace.ts, webs/src/views/network/Traceroute.vue and PathTopology.vue.
- [x] Add failing parser/options/compatibility tests, run RED.
- [x] Integrate bounded NextTrace structured output and explicit protocol/port/family, retaining traceroute fallback and legacy jobs.
- [x] Add source-specific vertical topology and ASN/location details, raw output and history.
- [x] Run focused Go tests and frontend checks.

## Task 2: Throughput runner
Files: throughputcore/* and agent_throughput_client.go (root integrates main.go).
- [x] Add failing args/result/budget/cleanup/auth tests, run RED.
- [x] Implement temporary RSA-authenticated iperf3 server, client JSON/interval results, process cleanup and leases.
- [x] Implement Agent desired-job poll reconciliation; bounded prepare/install action only on user request.
- [x] Verify localhost real TCP/UDP, reverse, bidirectional and invalid credentials where tool present.

## Task 3: Orchestration
Files: models/network_throughput.go, api/network_throughput.go, routers/network_trace.go, main.go, models/sqlite.go.
- [x] Add failing API tests for ownership/readiness/sequencing/cancellation/expiry.
- [x] Persist runs and source jobs; dispatch temporary server then client, report guarded by token and state, cancellation stops both sides, budget validation.
- [x] Add capability/prepare endpoints; register routes and migrations.
- [x] Run Go suite and race checks.

## Task 4: Throughput UI and menu
Files: webs/src/api/network/throughput.ts, webs/src/views/network/Throughput.vue, navigation.ts, api/mentu.go and menu tests.
- [x] Implement managed endpoint picker, one-to-one/batch controls, visible traffic budget and rate cap, result ranking/curves/history/cancel and path drilldown.
- [x] Place both features in network diagnosis, preserve node diagnostic menu.
- [x] Verify frontend regressions/typecheck/build and desktop/mobile interactions.

## Task 5: Integration and release
- [x] Review complete diff, full Go and frontend checks; independently review state machine/security.
- [ ] Commit only authorized changes with existing relay dependencies preserved; push and deploy using current documented release process.
- [ ] Backup consistent DB/config/executable before deployment, verify HTTP/auth/assets/Agent capabilities and bounded real inter-Agent test, preserve business configuration.

## Ledger
- Baseline full go test ./... passed 2026-10-29. Prior working-tree hashes and changed-file copies in /root/.cache/subl-network-lab-baseline.
- Ruling: existing deployed relay/delivery changes retained in place; user explicitly authorized implementation/push/deployment. No additional design approval needed.

- Task 1 complete: pinned NextTrace v1.7.4 structured schema verified against real binary; official asset API fallback, checksum installer, GeoIP outage fallback; source-specific topology, managed landing selector, 390/320px browser flow checked.
- Task 2 complete: iperf3 3.22 RSA OAEP, all TCP/UDP directions and IPv6 real tests, invalid auth and parent-death cleanup, bounded output and report retry/progress verified.
- Task 3 complete: ownership/readiness/endpoint reservation/lease expiry/cancel/prepare-nonce tests; 15s cooldown prevents immediate canceled-port reuse.
- Task 4 complete: frontend tests34 pass; typecheck/build and desktop/mobile interaction check passed. Final frontend rebuild follows last selector edit.
- Ruling: one RSA keypair per batch with unique random user/password per test; avoids repeated expensive key generation. Secrets remain hidden and are erased at completion/cancel/timeout.
- Prior deployed relay/delivery snapshot preserved as separate commit10198a3 before this feature, without changing working files.
