# Agent node deployment baseline

Agent node deployment follows the 3x-ui v2.9.3 implementation distributed by V2RaySSR/3x-ui-cn-installer. Reference commit0b5c239f98fd112df10ed4846377563a391ebf60. Sources: web/assets/js/model/inbound.js, web/service/xray.go, sub/subService.go. Read reference data/code only; never execute the external installer on managed hosts.

Implement on branch agents. Current runtime release is1.13 / capability6. Agent1.12 established this deployment profile;1.13 optimizes collection and adds client compatibility policies. Reality uses VLESS+raw+Vision, explicit target/serverNames/X25519/shortIds, default dual stack listen, chrome fingerprint and spiderX=/ in exported links. Permit importing an existing x-ui private key and short IDs, deriving/validating its public key. Existing stable identities remain unless the user explicitly replaces them.

New HY2 defaults to self-signed mode: no domain ownership, no certificate upload/paths. Agent persists generated certificate privately; client links explicitly contain insecure=1 like x-ui. TLS remains enabled because HY2 requires it. Existing auto/pinned mode remains pinned and is never silently downgraded. Manual trusted certificate mode remains available. Consistent skip verification in all supported subscription formats and managed server egress is required.

No extra test suite/traffic probes per user's preference. Build production assets/executable. Push only branch agents. Preserve existing ports, keys, credentials and business data while deploying; do not alter the user's separate x-ui installation or execute a second panel installer.

Future node deployment work must consult this pinned x-ui baseline and update this document deliberately when upstream changes.
