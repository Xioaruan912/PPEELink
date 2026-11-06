# Network unlock integration implementation plan

> **For agentic workers:** Use superpowers:executing-plans to implement task by task; final independent review.

**Goal:** Expand selected-node service checks and add browser-route checks without attributing browser results to a node.

**Architecture:** Reuse the existing Go sing-box/SOCKS runner and Vue test page. A read-only catalog endpoint supplies the same curated probe definitions to browser and backend. Preserve existing JSON clients; negotiate SSE for incremental results. Results distinguish API/page evidence, service-specific regional inference, reachability and unknown failures.

**Tech Stack:** Existing Go net/http, Gin, sing-box, Vue 3, fetch/AbortController. No new runtime dependencies or database migration.

**Spec:** User-approved research recommendation in this conversation (2026-11-05).

## Global constraints

- Preserve existing deployed, uncommitted changes in this linked worktree.
- Curate 18 common services; use upstream e941ebde9413ee844ecd34e8ea4ed8ccf7ac0a43 probe definitions/country lists with MIT attribution.
- Never treat challenge pages, generic errors, timeouts or no-cors opaque responses as confirmed unlock.
- Region inference uses only that service's own reported region, never a majority country.
- Backend and browser each cap concurrent service checks at 4, each service at 8 seconds, whole run at 45 seconds; cancellation aborts pending requests.
- Keep browser results in component memory only; never save them as selected-node observations.
- No account login, real playback/generation verification, 154-service full sweep, hosted iframe, headless browser, IPv4/IPv6 forcing or Agent changes.

## Review focus

- Unknown or malformed API responses and HTTP 403/429/5xx stay unknown.
- Empty/cancelled runs must not appear successful or poison history/cache.
- Component deactivation, switching source/category/node, and quick reruns cannot display stale results.
- SSE disconnects and callbacks terminate safely without concurrent response writes.
- Browser CORS failures may fall back to reachability, retaining honest evidence labels.

### Task 1: Node detectors and execution

Files: node/unlock.go, node/unlock_checks.go, node/unlock_catalog.go, node/unlock_evidence_test.go, existing unlock tests; THIRD_PARTY_NOTICES.md.
Interfaces: UnlockCheckResult adds evidence; status includes inferred/region_blocked/reachable. UnlockTestConfig adds GroupFilter and OnResult; ListUnlockServices returns stable complete catalog.

- [x] Write deterministic fake-transport tests for API/challenge failures, Netflix failure, region evidence, reach-only results and complete registry enumeration; run RED.
- [x] Add catalog and parsers, bounded body reads; implement controlled runner with cancellation, whole-run deadline and per-item callbacks.
- [x] Run focused Go tests; verify with local HTTP and a real local Shadowsocks proxy. Race checks unavailable because CGO/compiler is absent.

### Task 2: API and UI

Files: api/map_ping.go, routers/node.go, api/unlock_test.go; webs/src/components/node-tests/{unlock.ts,UnlockResults.vue,browserUnlock.ts,browserUnlock.test.mjs}; webs/src/views/test/{unlock.vue,DeviceUnlock.vue,index.vue}; webs/src/components/NodeUnlockDialog.vue; existing API client.
Interfaces: GET /nodes/unlock/services exposes catalog; POST /nodes/unlock accepts optional group and SSE Accept; old JSON response remains supported. Browser runner accepts catalog, AbortSignal and callback; emits existing result format with source browser.

- [x] Add failing tests for API validation/filtering, browser classifications, concurrency, deadline/cancellation and stale-run protection.
- [x] Implement catalog route and optional SSE; cache only complete results and isolate category keys. Update current and dialog clients.
- [x] Build a reusable probe runner and device pane; category controls, incremental rows, stop/error/empty states, provenance labels and source explanation.
- [x] Run frontend tests, typecheck and production build, focused API tests.

### Task 3: Review and delivery

- [x] Review only this feature's diff independently; address important findings with regression tests.
- [x] Check build artifact, meaningful local browser flows and existing focused tests.
- [x] Record commands, outcomes and limitations here; report deployment status accurately.

## Execution record

- Planning: accepted recommendation supplies scope; routine implementation details settled here, no repeated approval needed. Existing linked worktree used. No changes to Agent versions or node configuration.

- Backend verification: `go test ./node ./api -run '^TestUnlock|^TestBuildOutbound' -count=1` passed, including real local proxy routing, config parsing, bounded execution, cancellation, catalog/filter validation, JSON/SSE compatibility, incomplete cache exclusion and cached identity preservation.
- Frontend verification: all 54 existing/new Node tests passed; `pnpm run build` (Vite production + vue-tsc) passed. Browser fixture tests verified18 service rows, evidence labels, 320/768/1024/1440px, cancel, deactivation, no browser writes to node storage, and selected-node/category SSE handling.
- Independent review: fixed generated config JSON, cached node identity and legacy evidence labels; added regression coverage. No outstanding blocker. Source and source/category isolation preserved.
- Broader existing `TestHY2GatewayEditPreservesIngressCredentialsAndQuota` remains a known baseline failure already recorded by terminal1.16 deployment; no change to gateway logic. No full-suite success claim; race test unavailable in this CGO-disabled environment.
- Isolated candidate HTTP startup passed: authenticated18-service catalog, invalid filters rejected, SSE start/error flow and session release, embedded frontend assets. Candidate version `2026.11.05-unlock.0d737a4`; Agent remains1.16.
- Delivery evidence is in sibling `artifacts/unlock-integration/`; deployment result will be recorded after cutover.

- Deployed successfully: 2026.11.05-unlock.0d737a4; backup `/root/ppeelink-backups/20261105T080823Z-unlock`; cutover 1.37s; 3 live assets matched; all9 existing active node links and resource counts unchanged; database quick_check ok, public status operational. Agent1.16 unchanged. Public verification recorded in artifacts.
