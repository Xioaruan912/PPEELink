import {
  ref,
  reactive,
  computed,
  onMounted,
  onBeforeUnmount,
  provide,
  inject,
  type InjectionKey,
} from "vue";
import { createClashPreview } from "./preview";
import { useRouter } from "vue-router";
import { BuildClashTemplate, GetDefaultTemplate } from "@/api/template/build";
import { getTemp } from "@/api/template/temp";

interface GroupItem {
  name: string;
  type: string;
  filter: string;
  include_all_providers: boolean;
}
interface RuleProviderItem {
  name: string;
  url: string;
  behavior: string;
  type: string;
  interval: number;
  format: string;
  proxy: string;
  path: string;
}

function createTemplateBuilder() {
  const loading = ref(false);
  const sourceLoading = ref(false);
  const sourceError = ref("");
  const buildError = ref("");
  const savedSnapshot = ref("");
  const savedRevision = ref(0);
  const yamlPreview = ref("");
  const savedFilename = ref("");
  const templateSource = ref("default");
  const templateFiles = ref<any[]>([]);
  const editingOldname = ref("");
  const target = ref("clash"); // clash / loon
  const router = useRouter();

  // Loon 表单默认值（基础配置）
  const defaultGeneral = () => ({
    ip_mode: "v4-only",
    dns_server: "system",
    sni_sniffing: true,
    disable_stun: true,
    allow_wifi_access: false,
    wifi_http_port: 7222,
    wifi_socks_port: 7221,
    test_timeout: 5,
    switch_node_after_failure: 3,
    internet_test_url: "https://www.youtube.com",
    proxy_test_url: "https://www.youtube.com",
    resource_parser:
      "https://github.com/sub-store-org/Sub-Store/releases/latest/download/sub-store-parser.loon.min.js",
    geoip_url: "https://github.com/Masaiki/GeoIP2-CN/raw/release/Country.mmdb",
    ipasn_url:
      "https://raw.githubusercontent.com/P3TERX/GeoLite.mmdb/download/GeoLite2-ASN.mmdb",
    udp_fallback_mode: "REJECT",
    domain_reject_mode: "DNS",
    dns_reject_mode: "LoopbackIP",
    bypass_tun:
      "10.0.0.0/8,100.64.0.0/10,127.0.0.0/8,169.254.0.0/16,172.16.0.0/12,192.0.0.0/24,192.0.2.0/24,192.88.99.0/24,192.168.0.0/16,198.51.100.0/24,203.0.113.0/24,224.0.0.0/4,255.255.255.255/32",
    skip_proxy:
      "192.168.0.0/16,10.0.0.0/8,172.16.0.0/12,localhost,*.local,e.crashlynatics.com",
  });
  const defaultFilters = () => [
    {
      name: "美国",
      regex:
        "(?i)(美国|波特兰|达拉斯|俄勒冈|凤凰城|费利蒙|硅谷|拉斯维加斯|洛杉矶|圣何塞|圣克拉拉|西雅图|芝加哥|🇺🇸|US|USA)",
    },
    { name: "香港", regex: "(?i)(香港|hong|HK|HKG|🇭🇰)" },
    { name: "日本", regex: "(?i)(日本|东京|大阪|泉日|埼玉|🇯🇵|JP|Japan)" },
    { name: "新加坡", regex: "(?i)(狮城|新加坡|🇸🇬|SG|Singapore)" },
    { name: "台湾", regex: "(?i)(台湾|台灣|🇹🇼|TW|Taiwan)" },
  ];
  const defaultGroups = () => {
    const testURL = "http://cp.cloudflare.com/generate_204";
    const icon = "https://raw.githubusercontent.com/erdongchanyo/icon/main/";
    return [
      {
        name: "Global",
        type: "select",
        policies: "US, HK, JP, SG, DIRECT",
        url: testURL,
        img_url: icon + "Policy-Filter/Outside.png",
      },
      {
        name: "US",
        type: "select",
        policies: "美国",
        url: testURL,
        img_url: icon + "Policy-Country/US.png",
      },
      {
        name: "HK",
        type: "select",
        policies: "香港",
        url: testURL,
        img_url: icon + "Policy-Country/HK02.png",
      },
      {
        name: "JP",
        type: "select",
        policies: "日本",
        url: testURL,
        img_url: icon + "Policy-Country/JP.png",
      },
      {
        name: "SG",
        type: "select",
        policies: "新加坡",
        url: testURL,
        img_url: icon + "Policy-Country/SG.png",
      },
      {
        name: "TW",
        type: "select",
        policies: "台湾",
        url: testURL,
        img_url: icon + "Policy-Country/TW.png",
      },
      {
        name: "Netflix",
        type: "select",
        policies: "Global,TW, US, HK, JP, SG, DIRECT",
        url: testURL,
        img_url: icon + "Policy-Filter/Netflix.png",
      },
      {
        name: "Youtube",
        type: "select",
        policies: "Global, TW,HK, US, JP, SG, DIRECT",
        url: testURL,
        img_url: icon + "Policy-Filter/Youtube.png",
      },
      {
        name: "Mainland",
        type: "select",
        policies: "DIRECT",
        url: testURL,
        img_url: icon + "Policy-Filter/Mainland.png",
      },
      {
        name: "Advertising",
        type: "select",
        policies: "REJECT",
        url: testURL,
        img_url: icon + "Policy-Filter/AdBlock.png",
      },
      {
        name: "FINAL",
        type: "select",
        policies: "Global, DIRECT",
        url: testURL,
        img_url: icon + "Policy-Filter/Final01.png",
      },
    ];
  };

  const loonForm = ref({
    filename: "loon.conf",
    general: defaultGeneral(),
    filters: defaultFilters(),
    groups: defaultGroups(),
    remote_rules: "",
    plugins: "",
    rules: "",
  });

  // 前端生成 Loon 预览（与后端 buildLoonConfig 一致）
  const loonPreview = computed(() => {
    const g = loonForm.value.general;
    const lines: string[] = [];
    lines.push(
      "[General]",
      `ip-mode = ${g.ip_mode}`,
      `dns-server = ${g.dns_server}`,
      `sni-sniffing = ${g.sni_sniffing}`,
      `disable-stun = ${g.disable_stun}`,
      `udp-fallback-mode = ${g.udp_fallback_mode}`,
      `domain-reject-mode = ${g.domain_reject_mode}`,
      `dns-reject-mode = ${g.dns_reject_mode}`,
      `wifi-access-http-port = ${g.wifi_http_port}`,
      `wifi-access-socks5-port = ${g.wifi_socks_port}`,
      `allow-wifi-access = ${g.allow_wifi_access}`,
      "interface-mode = auto",
      `test-timeout = ${g.test_timeout}`,
      `switch-node-after-failure-times = ${g.switch_node_after_failure}`,
      `internet-test-url = ${g.internet_test_url}`,
      `proxy-test-url = ${g.proxy_test_url}`,
      `resource-parser = ${g.resource_parser}`,
      `geoip-url = ${g.geoip_url}`,
      `ipasn-url = ${g.ipasn_url}`,
      `skip-proxy = ${g.skip_proxy}`,
      `bypass-tun = ${g.bypass_tun}`,
      "",
      "[Proxy]",
      "",
      "[Remote Proxy]",
      "",
      "[Remote Filter]",
      "# 远程节点订阅正则筛选"
    );
    loonForm.value.filters.forEach(
      (f: any) =>
        f.name && lines.push(`${f.name} = NameRegex, FilterKey = "${f.regex}"`)
    );
    lines.push("", "[Proxy Group]", "");
    loonForm.value.groups.forEach((gr: any) => {
      if (!gr.name) return;
      let line = `${gr.name} = ${gr.type}, ${gr.policies}`;
      if (gr.url) line += `, url = ${gr.url}`;
      if (gr.img_url) line += `, img-url = ${gr.img_url}`;
      lines.push(line);
    });
    lines.push(
      "",
      "[Remote Rule]",
      loonForm.value.remote_rules,
      "",
      "[Proxy Chain]",
      "[Rule]",
      loonForm.value.rules,
      "DOMAIN-SUFFIX,local,DIRECT",
      "GEOIP,CN,DIRECT",
      "FINAL,FINAL",
      "",
      "[Host]",
      "",
      "[Rewrite]",
      "",
      "[Script]",
      "",
      "[Plugin]",
      loonForm.value.plugins,
      "",
      "[Mitm]",
      "hostname =",
      "skip-server-cert-verify = false"
    );
    return lines.join("\n");
  });

  const form = reactive({
    filename: "mihomo_default.yaml",
    port: 7890,
    socks_port: 7891,
    mixed_port: 7892,
    redir_port: 7893,
    tproxy_port: 7894,
    dns_listen_port: 1053,
    external_controller: "0.0.0.0:9090",
    port_offset: true,
    allow_lan: true,
    ipv6: true,
    udp: true,
    mode: "rule",
    log_level: "info",
    global_client_fingerprint: "chrome",
    unified_delay: true,
    geodata_mode: false,
    geo_auto_update: true,
    geo_update_interval: 24,
    tcp_concurrent: true,
    find_process_mode: "strict",
    sniffer_enable: true,
    tun_enable: true,
    tun_stack: "system",
    dns_enable: true,
    dns_enhanced_mode: "fake-ip",
    test_url: "http://www.gstatic.com/generate_204",
    interval: 300,
    groups: [] as GroupItem[],
    rule_providers: [] as RuleProviderItem[],
  });

  const rulesText = ref("");

  let alive = true;
  let sourceRequest = 0;
  onBeforeUnmount(() => {
    alive = false;
    sourceRequest++;
  });
  const loadDefault = async () => {
    if (!alive || templateSource.value !== "default") return;
    const request = ++sourceRequest;
    const initialSnapshot = snapshot.value;
    sourceLoading.value = true;
    sourceError.value = "";
    try {
      const { data } = await GetDefaultTemplate();
      if (
        !alive ||
        request !== sourceRequest ||
        templateSource.value !== "default" ||
        snapshot.value !== initialSnapshot
      )
        return;
      if (data) {
        Object.assign(form, data, { filename: "mihomo_default.yaml" });
        rulesText.value = (data.rules || []).join("\n");
      }
    } catch {
      if (alive && request === sourceRequest)
        sourceError.value = "默认模板加载失败，可重试或选择空白模板";
    } finally {
      if (alive && request === sourceRequest) sourceLoading.value = false;
    }
  };

  const onSourceChange = async (val: string) => {
    sourceRequest++;
    sourceLoading.value = false;
    sourceError.value = "";
    editingOldname.value = "";
    if (val === "default") {
      target.value = "clash";
      await loadDefault();
    } else if (val === "blank") {
      form.filename = "my_clash.yaml";
      form.groups = [
        {
          name: "🔰 节点选择",
          type: "select",
          filter: "",
          include_all_providers: false,
        },
      ];
      form.rule_providers = [];
      rulesText.value = "";
    } else if (val.startsWith("file:")) {
      const fname = val.slice(5);
      const item = templateFiles.value.find((t) => t.file === fname);
      if (item) {
        editingOldname.value = fname;
        form.filename = fname;
        // 判断是否为 Loon 配置（含 [Proxy]/[General] 段 或 .conf 后缀）
        const isLoon =
          fname.endsWith(".conf") ||
          /\[(General|Proxy|Remote Filter|Plugin)\]/.test(item.text || "");
        if (isLoon) {
          target.value = "loon";
          loonForm.value.filename = fname;
          // 简单解析已有文本：尽量填入表单（完整解析复杂，保留文本到 rules/remote_rules）
          loonForm.value.remote_rules = "";
          loonForm.value.plugins = "";
          loonForm.value.rules = "";
        } else {
          target.value = "clash";
          rulesText.value = item.text || "";
        }
      }
    }
  };

  const addGroup = () => {
    form.groups.push({
      name: "",
      type: "select",
      filter: "",
      include_all_providers: false,
    });
  };
  const removeGroup = (i: number) => form.groups.splice(i, 1);

  const addRP = () => {
    form.rule_providers.push({
      name: "",
      url: "",
      behavior: "classical",
      type: "http",
      interval: 3600,
      format: "yaml",
      proxy: "DIRECT",
      path: "./rules/",
    });
  };
  const removeRP = (i: number) => form.rule_providers.splice(i, 1);

  const build = async () => {
    const fname =
      target.value === "loon" ? loonForm.value.filename : form.filename;
    if (!fname.trim()) {
      ElMessage.warning("请填写文件名");
      return;
    }
    if (!alive || loading.value || sourceLoading.value) return;
    loading.value = true;
    buildError.value = "";
    const submittedSnapshot = snapshot.value;
    try {
      let payload: any;
      if (target.value === "loon") {
        payload = {
          target: "loon",
          filename: loonForm.value.filename.trim(),
          general: loonForm.value.general,
          filters: loonForm.value.filters,
          groups: loonForm.value.groups,
          remote_rules: loonForm.value.remote_rules,
          plugins: loonForm.value.plugins,
          rules: loonForm.value.rules,
        };
        if (editingOldname.value) payload.edit_oldname = editingOldname.value;
      } else {
        payload = {
          ...form,
          filename: form.filename.trim(),
          target: "clash",
          rules: rulesText.value
            .split("\n")
            .filter((l) => l.trim() && !l.trim().startsWith("#"))
            .map((l) => l.trim()),
        };
        if (templateSource.value === "default") payload.source = "default";
        if (editingOldname.value) payload.edit_oldname = editingOldname.value;
      }
      const { data } = await BuildClashTemplate(payload);
      if (!alive) return;
      yamlPreview.value = data?.yaml || "";
      savedFilename.value = data?.filename || "";
      savedSnapshot.value = submittedSnapshot;
      savedRevision.value += 1;
      ElMessage.success(`模板已保存：${savedFilename.value}`);
      loadTemplates();
    } catch (e: any) {
      buildError.value = e?.message || "生成失败";
      ElMessage.error(buildError.value);
    } finally {
      loading.value = false;
    }
  };

  const snapshot = computed(() =>
    JSON.stringify({
      target: target.value,
      source: templateSource.value,
      form,
      loon: loonForm.value,
      rules: rulesText.value,
    })
  );
  const isSavedPreview = computed(
    () => !!yamlPreview.value && savedSnapshot.value === snapshot.value
  );
  const preview = computed(() =>
    isSavedPreview.value
      ? yamlPreview.value
      : target.value === "loon"
        ? loonPreview.value
        : createClashPreview(form, rulesText.value)
  );

  const copyYaml = async () => {
    try {
      await navigator.clipboard.writeText(preview.value);
      ElMessage.success("已复制");
    } catch {
      ElMessage.warning("复制失败");
    }
  };

  const loadTemplates = async () => {
    try {
      const { data } = await getTemp();
      if (alive) templateFiles.value = data || [];
    } catch {
      if (alive) templateFiles.value = [];
    }
  };

  onMounted(async () => {
    await loadDefault();
    if (alive) await loadTemplates();
  });

  return {
    loading,
    sourceLoading,
    sourceError,
    buildError,
    savedFilename,
    savedRevision,
    isSavedPreview,
    templateSource,
    templateFiles,
    target,
    router,
    form,
    loonForm,
    rulesText,
    preview,
    loadDefault,
    onSourceChange,
    addGroup,
    removeGroup,
    addRP,
    removeRP,
    build,
    copyYaml,
  };
}
const builderKey: InjectionKey<ReturnType<typeof createTemplateBuilder>> =
  Symbol("template-builder");
export function provideTemplateBuilder() {
  const builder = createTemplateBuilder();
  provide(builderKey, builder);
  return builder;
}
export function useTemplateBuilder() {
  const builder = inject(builderKey);
  if (!builder) throw new Error("Template builder context is missing");
  return builder;
}
