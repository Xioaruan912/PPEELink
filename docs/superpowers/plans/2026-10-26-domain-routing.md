# Server-side website routing implementation plan
Use superpowers:executing-plans inline, with final independent review.
Goal: ordered per-inbound domain-to-egress rules, preserving metering and fail-closed routing.
Spec: docs/superpowers/specs/2026-10-26-domain-routing-design.md
Stack: Go/GORM/SQLite, Xray 26.3.27, Vue/TypeScript.
Constraints: 32 rules; exact/suffix domain semantics; capability 3; no automatic production routing changes; no fallback on selected-exit failure.
Review focus: matching boundaries/IDNA, overlaps and first match, cyclic exits through conditional edges, missing/paused nodes, metering and private-destination protection.
- [x] Core tests RED then GREEN: domain validation and rule order; collector route identities preserve inbound accounting and use selected bridge; existing no-rule path unchanged. Files xraycore/domain_routes.go, domain_routes_test.go, egress.go, website_collector.go, spec.go.
- [x] API tests RED then GREEN: saved normalized rules, old-Agent gate, rule-exit cycles, absent node rejection. Files models/proxy.go, api/proxy_domain_routes.go, proxy_egress.go, proxy_api.go, proxy_runtime.go, proxy_domain_routes_test.go.
- [x] Responsive rule editor: ordered add/remove/move, exact/suffix switch, exit selector, fallback explanation, clone drafts. Files webs/src/api/proxy/index.ts, servers/DomainRouteFields.vue, servers/InboundDrawer.vue.
- [x] Real Xray tests: matched domain selected exit, unmatched default, wrong suffix not matched, dead exit no direct fallback, with and without metering. Run full Go, Vue typecheck, frontend regressions/build; final review.
- [x] Commit/push/deploy, upgrade five Agents, verify capability 3 and original four inbounds unchanged; record release.

Verification: full Go suite, all real Xray feature tests, Vue typecheck, 20 frontend regressions and production build passed. Actual component checked at 390px iframe width with a screenshot; rule move updates bound order. Independent final review found no release-blocking issue. Real domain-routing tests replace the generated transport with isolated marker exits to distinguish selected/default/dead routes; existing real VLESS transport regression also passes.

Release 2026.10.26-9efd750 deployed. On 2026-10-27, HTTPS assets, API, schema and database integrity were rechecked; all five Agents reported capability 3 and the deployed version, with four enabled and running and WEB disabled. Four existing inbounds retained without domain rules.
