package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"ppeelink/models"
)

// runProbeAdd 在服务器上创建探针并打印一次性令牌与 systemd 安装命令。
func runProbeAdd(args []string) {
	fs := flag.NewFlagSet("probe-add", flag.ExitOnError)
	name := fs.String("name", "", "探针名称，例如 NoSLa_JP")
	region := fs.String("region", "", "地区，例如 JP")
	panel := fs.String("panel", "", "面板地址，例如 https://proxy.example.com")
	_ = fs.Parse(args)
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*panel) == "" {
		log.Fatal("需要 -name 与 -panel")
	}
	token := models.GenerateToken()
	probe := models.Probe{
		Name: strings.TrimSpace(*name), Region: strings.TrimSpace(*region),
		TokenHash: models.HashProbeToken(token), Enabled: true,
	}
	if err := models.DB.Create(&probe).Error; err != nil {
		log.Fatal("创建探针失败: ", err)
	}
	base := strings.TrimSuffix(*panel, "/")
	fmt.Printf("探针已创建: id=%d name=%s\n", probe.ID, probe.Name)
	fmt.Printf("令牌（只显示一次）: %s\n\n", token)
	fmt.Printf(`curl -fsSL -H 'Authorization: Probe %s' '%s/api/v1/probe/binary' -o /usr/local/bin/ppeelink-probe
chmod +x /usr/local/bin/ppeelink-probe
cat >/etc/systemd/system/ppeelink-probe.service <<'UNIT'
[Unit]
Description=PPEELink probe
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/ppeelink-probe probe -panel '%s' -token '%s' -interval 5m
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now ppeelink-probe
`, token, base, base, token)
}
