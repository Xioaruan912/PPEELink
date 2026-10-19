export function extractHostPort(
  link: string
): { host: string; port: number } | null {
  if (!link) return null;
  link = link.trim();

  if (link.startsWith("vmess://")) {
    try {
      const raw = link.slice(8);
      const decoded = atob(raw);
      const json = JSON.parse(decoded);
      return { host: json.add, port: parseInt(json.port) };
    } catch {
      return null;
    }
  }

  try {
    const url = new URL(link);
    const host = url.hostname;
    const port = parseInt(url.port);
    if (host && port) {
      return { host, port };
    }
  } catch {}

  // Fallback string parsing
  try {
    let rest = link;
    if (rest.includes("://")) {
      rest = rest.split("://")[1];
    }
    if (rest.includes("@")) {
      rest = rest.split("@")[1];
    }
    if (rest.includes("#")) {
      rest = rest.split("#")[0];
    }
    if (rest.includes("?")) {
      rest = rest.split("?")[0];
    }
    rest = rest.split("/")[0];
    if (rest.includes(":")) {
      const parts = rest.split(":");
      const port = parseInt(parts[parts.length - 1]);
      const host = parts
        .slice(0, -1)
        .join(":")
        .replace(/[\[\]]/g, "");
      if (host && !isNaN(port)) return { host, port };
    }
  } catch {}
  return null;
}
