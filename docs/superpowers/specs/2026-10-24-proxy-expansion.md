# Proxy expansion design
User approved starting fixed server egress, self-hosted Hysteria2 and application routing after feasibility review.
- Fixed egress: per inbound selects a stored supported node; default remains direct. Maintain TCP/UDP website payload accounting and private-target protection. Reject invalid/unsupported/cyclic/self references. Never silently fall back to direct. No changes to existing production routing selections.
- Hysteria2: reuse managed Xray v26.3.27 where compatible. Add explicit TLS certificate/key file paths (on target host), per-account authentication, subscriptions, safe validation. Existing Reality behavior unchanged. Validate real core in isolation. Keep credentials out of error logs.
- Application routing: explicit desktop process names/Android packages in full client configurations; do not promise iOS per-app or support in bare node links. Ordered rules target existing groups/DIRECT/REJECT, validated input. Preserve old templates.
- UI: neutral gray/white, small blue accents, mobile-friendly existing forms.
- Deliver tested code and deploy using established release procedure. Do not enable new production routes/listeners without user choice of endpoints/certificate paths.
