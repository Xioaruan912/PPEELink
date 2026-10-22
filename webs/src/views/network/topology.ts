export type TopologyKind = "subscription" | "group" | "node" | "inbound" | "server" | "monitor";
export interface TopologyNode {
  id: string;
  kind: TopologyKind;
  name: string;
  status: string;
  detail?: string;
  route: string;
  checkedAt?: string;
}
export interface TopologyEdge { source: string; target: string; relation: string }
export interface TopologySnapshot {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
  warnings: string[];
  generatedAt: string;
}
export const kindLabels: Record<TopologyKind, string> = {
  subscription: "订阅", group: "分组", node: "节点", inbound: "自建入站", server: "代理服务器", monitor: "服务器监控",
};
export const relationLabels: Record<string, string> = {
  fixed: "固定节点", reference: "引用分组", member: "分组成员", relay: "经由中转", server: "入站所属服务器", monitor: "关联监控",
};
export const statusLabels: Record<string, string> = {
  configured: "已配置", unknown: "未检测", hidden: "已隐藏", paused: "已暂停", expired: "已过期", disabled: "已停用", stopped: "未运行", running: "运行中", online: "在线", offline: "离线",
};
export const columnKinds: TopologyKind[][] = [["subscription"], ["group"], ["node", "inbound"], ["server"], ["monitor"]];

export function normalizeGraph(snapshot: TopologySnapshot) {
  const nodes = [...new Map(snapshot.nodes.map(n => [n.id, n])).values()];
  const ids = new Set(nodes.map(n => n.id));
  const unique = [...new Map(snapshot.edges.map(e => [`${e.source}|${e.target}|${e.relation}`, e])).values()];
  return { nodes, edges: unique.filter(e => ids.has(e.source) && ids.has(e.target)), dangling: unique.filter(e => !ids.has(e.source) || !ids.has(e.target)) };
}

// Trace upstream and downstream independently: unlike an undirected component,
// selecting one subscription does not pull in every sibling subscription/group.
export function relatedIds(seeds: Set<string>, edges: TopologyEdge[]) {
  const walk = (reverse: boolean) => {
    const found = new Set(seeds);
    const adjacency = new Map<string, string[]>();
    for (const e of edges) {
      const from = reverse ? e.target : e.source;
      const to = reverse ? e.source : e.target;
      adjacency.set(from, [...(adjacency.get(from) || []), to]);
    }
    const queue = [...seeds];
    for (let i = 0; i < queue.length; i++) {
      for (const next of adjacency.get(queue[i]) || []) {
        if (!found.has(next)) { found.add(next); queue.push(next); }
      }
    }
    return found;
  };
  return new Set([...walk(false), ...walk(true)]);
}

export function filterGraph(snapshot: TopologySnapshot, search = "", focus = "", perColumnLimit = 36) {
  const graph = normalizeGraph(snapshot);
  const query = search.trim().toLocaleLowerCase();
  const matches = new Set(graph.nodes.filter(n => !query || `${n.name} ${n.id} ${kindLabels[n.kind]}`.toLocaleLowerCase().includes(query)).map(n => n.id));
  let allowed = query ? relatedIds(matches, graph.edges) : new Set(graph.nodes.map(n => n.id));
  if (focus) {
    const related = relatedIds(new Set([focus]), graph.edges);
    allowed = new Set([...allowed].filter(id => related.has(id)));
  }
  const candidates = graph.nodes.filter(n => allowed.has(n.id));
  const columns = columnKinds.map(kinds => candidates.filter(n => kinds.includes(n.kind)).sort((a, b) => {
    const priority = (n: TopologyNode) => n.id === focus ? 2 : query && matches.has(n.id) ? 1 : 0;
    return priority(b) - priority(a) || a.id.localeCompare(b.id, undefined, { numeric: true });
  }).slice(0, perColumnLimit));
  const visible = new Set(columns.flat().map(n => n.id));
  return { columns, edges: graph.edges.filter(e => visible.has(e.source) && visible.has(e.target)), dangling: graph.dangling, total: graph.nodes.length, matched: matches.size, filtered: candidates.length, shown: visible.size };
}
