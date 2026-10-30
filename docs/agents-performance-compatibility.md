# Agent 1.13: collection performance and client compatibility

## Scope

Continue on branch `agents`. Replace per-tick Xray API subprocesses with a reused loopback gRPC connection. Bound online-IP lookup concurrency to four and the whole collection to15seconds; destructive/reset requests have no application retry. Close the connection on stop/restart/port changes, cache the binary version by size+mtime and service status for two seconds. Preserve existing reporting, quotas, node identities and proxy configuration.

Compatibility uses one server-side policy for export filtering and pre-export previews. Parse each input once when generating the cross-client report. Reports contain node IDs/names/protocols and actionable reasons, never credentials or links. The node page supports all/selected nodes; subscription sharing uses the delivered group/pipeline result and selected client. These checks describe exporter capability and version conditions, not live connectivity.

Distinct profiles are retained for Stash/Egern/Surfboard rather than treating a shared config syntax as full protocol compatibility. Unsupported required parameters are excluded, zero-compatible native exports return422, and artifacts use a compatibility revision to prevent falling back to outputs from an old policy. Original generic share-link exports remain transparent and clearly marked version-dependent.

Reality validation covers key/SNI/shortID/flow/transport. Preserve HY2 trust mode; Surge now exports its documented server-cert-fingerprint-sha256 instead of dropping pinned nodes. sing-box retains explicit verification preferences while pins/Reality cannot be downgraded by global skip flags. Future native protocol additions need matching export and policy changes.

## Sources

- https://github.com/MHSanaei/3x-ui/blob/v2.9.3/xray/api.go
- https://github.com/XTLS/Xray-core/blob/v26.3.27/app/stats/command/command.proto
- https://github.com/XTLS/Xray-core/blob/v26.3.27/main/commands/all/api/stats_online_ip_list.go
- https://manual.nssurge.com/policies/overview.html
- https://manual.nssurge.com/policies/hysteria2.html
- https://manual.nssurge.com/policies/tls.html
- https://sing-box.sagernet.org/configuration/shared/v2ray-transport/

## Delivery

Agent version1.13. Production Go/frontend builds and deployment acknowledgements only, following the user's standing instruction to skip additional tests and network probes. No measured CPU/latency improvement is asserted without a benchmark.

## Review

Independent read-only review covered gRPC wire requests/counter names, native exporters, alias/provider/fallback paths, and personal/temporary subscription behavior. Corrected temporary alias output, preserved VMess Alter ID where supported and excluded unsupported authentication variants, excluded SS plugin links where required plugin fields would be lost. Follow-up review found no remaining high/medium issue in those paths.
