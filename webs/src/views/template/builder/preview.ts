/** Form-only YAML preview. The server completes fixed defaults when saving. */
export function createClashPreview(
  form: Record<string, any>,
  rulesText: string
) {
  const offset = form.port_offset && form.port ? form.port - 7890 : 0;
  const port = (value: number, initial: number) =>
    value === initial ? value + offset : value;
  const config: Record<string, unknown> = {
    port: form.port,
    "socks-port": port(form.socks_port, 7891),
    "mixed-port": port(form.mixed_port, 7892),
    "redir-port": port(form.redir_port, 7893),
    "tproxy-port": port(form.tproxy_port, 7894),
    "allow-lan": form.allow_lan,
    mode: form.mode,
    "log-level": form.log_level,
    ipv6: form.ipv6,
    udp: form.udp,
    "external-controller": form.external_controller,
    "unified-delay": form.unified_delay,
    "geodata-mode": form.geodata_mode,
    "geo-auto-update": form.geo_auto_update,
    "geo-update-interval": form.geo_update_interval,
    "tcp-concurrent": form.tcp_concurrent,
    "find-process-mode": form.find_process_mode,
  };
  if (form.geodata_loader !== undefined)
    config["geodata-loader"] = form.geodata_loader;
  if (form.sniffer_enable) config.sniffer = { enable: true };
  if (form.tun_enable) config.tun = { enable: true, stack: form.tun_stack };
  if (form.dns_enable)
    config.dns = {
      enable: true,
      listen: `0.0.0.0:${form.dns_listen_port}`,
      "enhanced-mode": form.dns_enhanced_mode,
    };
  config["proxy-groups"] = form.groups
    .filter((group: any) => group.name)
    .map((group: any) => {
      const entry: Record<string, unknown> = {
        name: group.name,
        type: group.type,
        proxies: group.proxies?.length ? group.proxies : ["DIRECT"],
      };
      if (["url-test", "fallback"].includes(group.type)) {
        entry.url = form.test_url;
        entry.interval = form.interval;
      }
      if (group.filter) entry.filter = group.filter;
      if (group.filter || group.include_all_providers)
        entry["include-all-providers"] = !!group.include_all_providers;
      return entry;
    });
  const providers = Object.fromEntries(
    form.rule_providers
      .filter((provider: any) => provider.name)
      .map((provider: any) => [
        provider.name,
        Object.fromEntries(
          ["type", "behavior", "interval", "format", "proxy", "path", "url"]
            .filter(
              (key) => provider[key] !== "" && provider[key] !== undefined
            )
            .map((key) => [key, provider[key]])
        ),
      ])
  );
  if (Object.keys(providers).length) config["rule-providers"] = providers;
  config.rules = rulesText
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith("#"));
  return "# 表单实时预览；保存时由服务端补全内置配置\n" + yaml(config) + "\n";
}

function scalar(value: unknown): string {
  return value === undefined ? "null" : JSON.stringify(value);
}
function key(value: string) {
  return /^[a-zA-Z0-9_-]+$/.test(value) ? value : JSON.stringify(value);
}
function yaml(value: unknown, depth = 0): string {
  const pad = "  ".repeat(depth);
  if (Array.isArray(value)) {
    if (!value.length) return "[]";
    return value
      .map((item) => {
        if (item && typeof item === "object") {
          const lines = yaml(item, depth + 1).split("\n");
          return (
            pad + "- " + lines[0].trimStart() + "\n" + lines.slice(1).join("\n")
          );
        }
        return pad + "- " + scalar(item);
      })
      .join("\n");
  }
  return Object.entries(value as Record<string, unknown>)
    .map(([name, item]) => {
      if (Array.isArray(item) && !item.length) return `${pad}${key(name)}: []`;
      if (item && typeof item === "object")
        return `${pad}${key(name)}:\n${yaml(item, depth + 1)}`;
      return `${pad}${key(name)}: ${scalar(item)}`;
    })
    .join("\n");
}
