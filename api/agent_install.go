package api

import (
	"fmt"
	"strings"
)

// Both roles install the panel executable. Keep download, credentials, atomic
// replacement and service restart in one template so upgrades cannot drift.
func agentInstallScript(panel, token, role string) string {
	auth, path := "Agent", "/agent/binary"
	if role == "monitor" {
		auth, path = "Monitor", "/api/v1/monitor/binary"
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	panel = strings.TrimRight(panel, "/")
	// systemd expands percent specifiers even in quoted arguments.
	unitPanel := strings.NewReplacer("%", "%%", "\\", "\\\\", "\"", "\\\"", "\n", "", "\r", "").Replace(panel)
	return fmt.Sprintf(`#!/bin/sh
set -eu
PANEL=%s
TOKEN=%s
ROLE=%s
[ "$(id -u)" = 0 ] || { echo "请用 root 运行"; exit 1; }
command -v systemctl >/dev/null || { echo "需要 systemd"; exit 1; }
case "$(uname -m)" in
 x86_64|amd64) ARCH=amd64 ;;
 aarch64|arm64) ARCH=arm64 ;;
 *) echo "不支持的架构: $(uname -m)"; exit 1 ;;
esac
BIN="/usr/local/bin/ppeelink-$ROLE"
TMP=$(mktemp "$BIN.XXXXXX")
trap 'rm -f "$TMP"' EXIT HUP INT TERM
curl -fsSL -H "Authorization: %s $TOKEN" "$PANEL%s?arch=$ARCH" -o "$TMP"
chmod 755 "$TMP"
mv -f "$TMP" "$BIN"
mkdir -p "/etc/ppeelink-$ROLE" /var/lib/ppeelink-agent
umask 077
printf 'PPEELINK_AGENT_TOKEN=%%s\n' "$TOKEN" > "/etc/ppeelink-$ROLE/env"
chmod 600 "/etc/ppeelink-$ROLE/env"
cat > "/etc/systemd/system/ppeelink-$ROLE.service" <<'UNIT'
[Unit]
Description=PPEELink %s
After=network-online.target
Wants=network-online.target

[Service]
EnvironmentFile=/etc/ppeelink-%s/env
ExecStart=/usr/local/bin/ppeelink-%s %s -panel "%s"
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable "ppeelink-$ROLE" >/dev/null 2>&1
systemctl restart "ppeelink-$ROLE"
echo "PPEELink $ROLE 已安装并启动，一分钟内会出现在面板中。"
`, quote(panel), quote(token), quote(role), auth, path, role, role, role, role, unitPanel)
}
