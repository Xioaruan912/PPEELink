import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import ts from "typescript";
const exports = {};
vm.runInNewContext(ts.transpileModule(
  readFileSync(new URL("./navigation.ts", import.meta.url), "utf8"),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }
).outputText, { exports, require: name => name === "@/utils/i18n"
  ? { translateRouteTitle: title => title } : { default: {} } });
const { buildNavigation, organizeNavigation, flattenNavigation } = exports;
const route = (path, title, children) => ({ path, meta: { title }, ...(children ? { children } : {}) });
const paths = items => Array.from(flattenNavigation(items), item => item.path);
test("groups permitted features by task while preserving every URL once", () => {
  const input = [
    route("/", "", [route("dashboard", "dashboard")]),
    route("/monitor", "monitor", [route("index", "monitor"), route("website", "website"), route("workspace", "workspace")]),
    route("/subcription", "subcription", [route("subs", "subs"), route("nodes", "nodes"), route("servers", "servers")]),
    route("/network", "network", [route("traceroute", "traceroute")]),
    route("/test", "test", [route("index", "test")]),
    route("/future", "Future"),
  ];
  const original = buildNavigation(input);
  const organized = organizeNavigation(original);
  assert.deepEqual(paths(organized).sort(), paths(original).sort());
  assert.equal(new Set(paths(organized)).size, paths(original).length);
  assert.equal(organized[0].label, "总览");
  assert.deepEqual(paths([organized.find(item => item.label === "服务器")]),
    ["/monitor/index", "/subcription/servers", "/monitor/workspace", "/monitor/website"]);
  assert.deepEqual(paths([organized.find(item => item.label === "网络诊断")]),
    ["/test/index", "/network/traceroute"]);
});
test("ordinary user routes never acquire admin links and hidden routes stay hidden", () => {
  const items = organizeNavigation(buildNavigation([
    route("/my", "我的", [route("subscriptions", "我的订阅")]),
    route("/system", "系统", [{ path: "user/set", meta: { title: "密码", hidden: true } }]),
  ]));
  assert.deepEqual(paths(items), ["/my/subscriptions"]);
});
test("missing permissions omit features and unknown future routes remain accessible", () => {
  const items = organizeNavigation(buildNavigation([
    route("/monitor", "monitor", [route("index", "monitor")]),
    route("/extra", "extra", [route("one", "one")]),
  ]));
  assert.deepEqual(paths(items), ["/monitor/index", "/extra/one"]);
});

test("diagnostic stays inside subscriptions instead of becoming a top-level link", () => {
 const items=organizeNavigation(buildNavigation([route("/subcription","订阅与节点",[route("subs","订阅列表"),route("nodes","节点列表"),route("diagnostic","节点一键诊断")])]));
 assert.equal(items.length,1);
 assert.equal(items[0].label,"订阅与节点");
 assert.deepEqual(paths(items),["/subcription/subs","/subcription/nodes","/subcription/diagnostic"]);
});
