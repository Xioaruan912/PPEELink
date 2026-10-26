# Website category routing plan
Inline execution. Spec: docs/superpowers/specs/2026-10-27-geosite-routing-design.md.
- [x] Test category allowlist/normalization and ordered config, no weakening domain validation; implement core and API with Agent capability 4.
- [x] Extend mobile rule editor with category selection and explicit database/version behavior.
- [x] Real Xray tests including category/noncategory and metered/unmetered, failure behavior; run Go/frontend verification and review.
- [x] Push/deploy and upgrade Agents; verify existing routing and service.

Validation: allowed tags accepted by Xray 26.3.27 using installed geosite file. Full Go suite, real TCP/UDP category and ordering tests (metered/unmetered), old-Agent rejection, and missing-data preservation passed; 20 frontend regressions and Vue typecheck passed. Independent review identified misleading regional coverage labels; clarified that these are lists and coverage may change. Release 2026.10.27-a7ef6ca deployed; all five Agents reported capability 4 and deployed version, four enabled and running, WEB disabled. HTTPS assets, API and database integrity verified. Existing business record counts preserved.
