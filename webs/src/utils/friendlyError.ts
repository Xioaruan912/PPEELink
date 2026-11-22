/**
 * Translates raw Go / sing-box / Xray / network error strings into concise,
 * human-friendly Chinese feedback so technical build/stack messages never leak to the UI.
 */
export interface FriendlyError {
  title: string;
  hint: string;
  tone: "warn" | "bad";
}

export function translateFriendlyError(raw?: string | null): FriendlyError {
  const msg = String(raw || "").trim();
  const lower = msg.toLowerCase();

  if (lower.includes("with_quic") || lower.includes("quic is not included")) {
    return {
      title: "当前环境暂不支持 QUIC / HY2 探测",
      hint: "当前运行的后端未启用 QUIC 模块，线上正式版已完整支持。",
      tone: "warn",
    };
  }
  if (lower.includes("with_utls") || lower.includes("utls is not included")) {
    return {
      title: "当前环境暂不支持 uTLS 指纹探测",
      hint: "当前后端未启用 uTLS 模块，请使用正式构建版本。",
      tone: "warn",
    };
  }
  if (lower.includes("context deadline exceeded") || lower.includes("timeout") || lower.includes("超时")) {
    return {
      title: "连接探测超时",
      hint: "目标节点或远端线路响应过慢，请稍后重试或切换其他节点。",
      tone: "warn",
    };
  }
  if (lower.includes("connection refused") || lower.includes("connect: no route")) {
    return {
      title: "目标端口未响应",
      hint: "节点服务可能未启动或防火墙拦截了当前端口。",
      tone: "bad",
    };
  }
  if (lower.includes("no such host") || lower.includes("dns")) {
    return {
      title: "域名解析失败",
      hint: "请检查节点域名拼写或 DNS 解析是否生效。",
      tone: "warn",
    };
  }
  if (lower.includes("sing-box") || lower.includes("initialize outbound")) {
    return {
      title: "节点链路初始化失败",
      hint: "该节点协议参数暂不兼容当前探测内核，请核对节点配置。",
      tone: "warn",
    };
  }
  if (lower.includes("err_canceled") || lower.includes("已取消")) {
    return {
      title: "检测已取消",
      hint: "你已主动停止本次任务。",
      tone: "warn",
    };
  }
  return {
    title: msg && msg.length <= 42 && !/[a-z]{5,}\[\d+\]/i.test(msg) ? msg : "本次检测未能完成",
    hint: "请确认节点在线且网络畅通后重试。",
    tone: "warn",
  };
}
