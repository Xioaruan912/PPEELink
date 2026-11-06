# Fix Agent throughput-tool preparation

User expects default readiness or one-click installation, not compilation dependency setup.

Cause: Prepare downloaded pinned source and ran cc/make/OpenSSL on each VPS; both affected Agents are current1.16, while the UI always suggests updating old Agents.

Implementation: reuse the existing NextTrace fixed-release download pattern for userdocs/iperf3-static3.22. Pin SHA256 for Linux amd64, arm64, ARMv7 (reject older unspecified ARM). Verify checksum before execution, exact version3.22 and RSA options before atomic installation; preserve existing valid tools and include licenses. No package manager or compilation on Agents. Agent1.17 prepares a missing tool once at startup, before ordinary polling; failed downloads remain retryable through existing prepare jobs. UI shows upgrade advice only for actually older Agents.

- [x] Regression tests: missing compiler, checksum/version failure preserving existing binary, API download fallback, unsupported architecture, cancellation/size limits.
- [x] Implement core download and startup preparation, accurate UI/API capability copy, documentation and attribution.
- [x] Run focused Go/frontend tests, real static-binary authentication/TCP/UDP measurements, production build and arm64/ARMv7 cross-build.
- [x] Independent review and targeted repairs.
- [x] Back up and deploy panel, canary DMIT_USA/HKCLOUD_HK Agent1.17, verify automatic readiness, then remaining Agents without changing enabled state or routing.

Evidence and rollout records: sibling artifacts/iperf-preparation. Existing uncommitted deployed changes are preserved.

Completed: all7 Agents are1.17; all7 have iperf3 3.22 RSA authentication ready; each enabled state preserved (WEB remains disabled). Static real tests passed for TCP/UDP forward/reverse/bidirectional, IPv6 and invalid credentials/cancellation;54 frontend tests, focused Go and production build passed. Panel version2026.11.06-agent-1.17.0d737a4; rollback /root/ppeelink-backups/20261106T005449Z-agent-1.17. Temporary validation authorization removed.
