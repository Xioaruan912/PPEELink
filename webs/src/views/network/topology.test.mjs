// Run: node --experimental-strip-types --test src/views/network/topology.test.mjs
import test from "node:test";
import assert from "node:assert/strict";
import { filterGraph, normalizeGraph, relatedIds } from "./topology.ts";
const node = (id, kind, name = id) => ({ id, kind, name, status: "configured", route: "/" });
const edge = (source, target, relation = "member") => ({ source, target, relation });
const data = {
  nodes: [node("s:1", "subscription", "Alpha"), node("s:2", "subscription", "Sibling"), node("g:1", "group"), node("n:1", "node"), node("n:2", "node"), node("server:1", "server", "Alpha")],
  edges: [edge("s:1", "g:1", "reference"), edge("s:2", "g:1", "reference"), edge("g:1", "n:1"), edge("n:1", "n:2", "relay"), edge("n:2", "n:1", "relay"), edge("n:1", "missing:1", "relay")],
  warnings: [], generatedAt: "2026-01-01T00:00:00Z",
};
test("dangling refs disclosed, never synthesized; duplicate edges removed", () => {
  const graph = normalizeGraph({ ...data, edges: [...data.edges, data.edges[0]] });
  assert.equal(graph.dangling.length, 1);
  assert.equal(graph.edges.length, 5);
  assert.equal(graph.nodes.length, data.nodes.length);
});
test("focus walks explicit ancestors/descendants and terminates cycles without sibling spill", () => {
  const ids = relatedIds(new Set(["s:1"]), normalizeGraph(data).edges);
  assert.deepEqual([...ids].sort(), ["s:1", "g:1", "n:1", "n:2"].sort());
  assert.equal(ids.has("s:2"), false);
  assert.equal(ids.has("server:1"), false);
});
test("search includes actual paths but does not invent same-name association", () => {
  const result = filterGraph(data, "Sibling");
  assert.equal(result.filtered, 4);
  assert.equal(result.columns.flat().some(n => n.id === "server:1"), false);
  assert.equal(filterGraph(data, "does-not-exist").shown, 0);
});
test("visible cap counts omissions, retains selected object, and clips edges", () => {
  const nodes = Array.from({ length: 80 }, (_, i) => node(`node:${i}`, "node"));
  const result = filterGraph({ ...data, nodes, edges: [edge("node:79", "node:0")] }, "", "node:79", 1);
  assert.equal(result.filtered, 2);
  assert.equal(result.shown, 1);
  assert.equal(result.columns[2][0].id, "node:79");
  assert.equal(result.edges.length, 0);
});
