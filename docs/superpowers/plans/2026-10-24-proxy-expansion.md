# Proxy Expansion Implementation Plan
> **For agentic workers:** Use superpowers:subagent-driven-development for independent tasks and review each diff.
**Goal:** Fixed egress, Hysteria2 listeners, application-routing full configs.
**Architecture:** Extend existing Go model/spec/config and Vue forms, reuse Agent runtime and client serializers.
**Tech Stack:** Go, SQLite/GORM, Xray, Vue/TypeScript.
**Spec:** docs/superpowers/specs/2026-10-24-proxy-expansion.md
## Global Constraints
Preserve existing direct routing and Reality. No automatic production feature enablement. Maintain website TCP/UDP metering and private-target protection. Unknown settings fail closed. Keep credentials out of logs. Existing neutral/mobile design.
## Review Focus
- Invalid exit selection, self/cyclic routes: reject before save and defensively during config generation.
- Exit failure: no direct fallback or unmetered traffic.
- Hysteria2 credentials/TLS: config validation and real isolated core test.
- App rule platform/target validation: no unsupported output silently accepted.
- Existing settings: zero-value backward compatibility and full tests.
## Task 1: Fixed egress
Files: models/proxy.go, api/proxy_egress.go, api/proxy_api.go, api/proxy_runtime.go, xraycore/spec.go/config.go/website_collector.go, Vue InboundDrawer and API types.
- [x] Write failing tests for egress validation, routing and TCP/UDP metering.
- [x] Implement per-inbound selection and fail-closed forwarding.
- [x] Run focused tests and isolated proxy test; review diff.
## Task 2: Hysteria2
Files: models/proxy.go, api/proxy_api.go/proxy_runtime.go, xraycore/spec.go/config.go, new Hysteria config helper and tests, InboundDrawer, API types.
- [x] Write failing tests for HY2 inbound validation and links.
- [x] Implement TLS file configuration, account auth, generated links and form.
- [x] Test installed real Xray in isolation; review diff.
## Task 3: Application routing
Files: subscription SqlConfig, client rendering and subscription editor; focused tests.
- [x] Find template persistence and client full-config serializers.
- [x] Test valid ordered process/package rules, unsupported targets/platforms.
- [x] Implement editor and supported full-config generation.
- [x] Run focused tests and review diff.
## Integration
- [x] Full Go tests, frontend typecheck/build/tests.
- [ ] Release and verify production assets/API after final build.

## Implementation and verification notes (2026-10-25)
- Application rules live in subscription configuration so both Mihomo and sing-box outputs share the same ordered rules. Supported desktop processes and Android packages; iOS is explicitly excluded.
- Xray v26.3.27 requires Hysteria `settings.clients` (confirmed in tagged source and real-core test), despite newer documentation using `users`. No-user listeners are omitted.
- Isolated real-core tests cover authenticated SOCKS bridge TCP/UDP, Hysteria TCP/UDP, wrong-password rejection, HY2 account counters, website records, and VLESS Reality fixed egress with/without website collection.
- Full Go suite passed. Frontend typecheck, 20 regression tests and Vite build passed; final build is repeated after the SNI form refinement.
- Browser fixture rendered actual components. Desktop HY2 certificate form checked. At 390px iframe width, app rules use one column and body scrollWidth equals clientWidth; moving a rule changes order. Mobile screenshot API timed out, so this portion used DOM geometry and accessibility evidence.
- Requested helper agents could not run due quota. Implementation and final diff review were completed by the primary agent; no independent review is claimed.
- Production listeners and routing remain unchanged until configured by the administrator. Hysteria requires valid TLS certificate/key paths on the target host and reachable UDP port.
