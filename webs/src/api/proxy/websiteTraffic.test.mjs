// No extra test dependencies: node --test src/api/proxy/websiteTraffic.test.mjs
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import vm from "node:vm";
import ts from "typescript";
import { parse, compileScript } from "vue/compiler-sfc";
import { createSSRApp, h } from "vue";
import { renderToString } from "vue/server-renderer";

const require = createRequire(import.meta.url);
const browser = { onLine: true };
function evaluate(source, imports, globals = {}) {
  const code = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const exports = {};
  vm.runInNewContext(code, {
    exports,
    require: (name) => imports[name] ?? require(name),
    Date,
    Intl,
    console,
    navigator: browser,
    ...globals,
  });
  return exports;
}
const datetime = evaluate(
  readFileSync(new URL("../../utils/datetime.ts", import.meta.url), "utf8")
);
let requested;
const api = evaluate(
  readFileSync(new URL("./websiteTraffic.ts", import.meta.url), "utf8"),
  {
    "@/utils/datetime": datetime,
    "@/utils/request": {
      default: async (config) => {
        requested = config;
        return { data: fixture };
      },
    },
  }
);
const query = {
  start: "2026-10-22T12:00:00+08:00",
  end: "2026-10-23T12:00:00+08:00",
};
const fixture = {
  items: [
    {
      serverId: 3,
      serverName: "测试节点",
      domain: "unknown",
      up: 12,
      down: 30,
      total: 42,
      rank: 1,
    },
  ],
  summary: { up: 12, down: 30, total: 42 },
  start: "2026-10-22T04:00:00Z",
  end: "2026-10-23T04:00:00Z",
  scope: "proxy",
  granularity: "hour",
  unknownDomain: "unknown",
};

test("GET client uses standard envelope, optional filters and RFC3339 defaults", async () => {
  const response = await api.getWebsiteTraffic(query);
  assert.equal(response.data, fixture);
  assert.equal(requested.url, "/api/v1/proxy/website-traffic");
  assert.equal(requested.method, "get");
  assert.equal(requested.params.order, "total");
  assert.equal(requested.params.limit, 50);
  assert.equal(requested.params.start, "2026-10-22T04:00:00.000Z");
  assert.equal("domain" in requested.params, false);
  assert.equal("serverId" in requested.params, false);
  await api.getWebsiteTraffic({
    ...query,
    serverId: 3,
    domain: " example.com ",
    order: "down",
    limit: 100,
  });
  assert.equal(requested.params.serverId, 3);
  assert.equal(requested.params.domain, "example.com");
  assert.equal(requested.params.order, "down");
});

test("invalid range, order, limit and server reject before requesting", () => {
  for (const invalid of [
    { start: "bad" },
    { end: query.start },
    { limit: 0 },
    { limit: 101 },
    { limit: 1.5 },
    { serverId: 0 },
    { order: "random" },
  ])
    assert.throws(() => api.websiteTrafficParams({ ...query, ...invalid }));
});

test("整点边界含秒/毫秒校验，并强制 366 天上限（时间按东八区解释）", () => {
  for (const start of [
    "2026-10-22T04:01:00Z",
    "2026-10-22T04:00:01Z",
    "2026-10-22T04:00:00.001Z",
    "2026-10-22T12:00:00+05:30",
  ]) {
    assert.throws(() => api.websiteTrafficParams({ ...query, start }), /整点/);
  }
  assert.throws(
    () =>
      api.websiteTrafficParams({ ...query, end: "2026-10-23T04:00:00.001Z" }),
    /整点/
  );
  const start = "2025-01-01T00:00:00Z";
  assert.doesNotThrow(() =>
    api.websiteTrafficParams({
      start,
      end: new Date(+new Date(start) + 366 * 86400000).toISOString(),
    })
  );
  assert.throws(
    () =>
      api.websiteTrafficParams({
        start,
        end: new Date(
          +new Date(start) + 366 * 86400000 + 3600000
        ).toISOString(),
      }),
    /366/
  );
  const halfHourZone = api.websiteTrafficParams({
    start: "2026-10-22T09:30:00+05:30",
    end: "2026-10-22T10:30:00+05:30",
  });
  assert.equal(halfHourZone.start, "2026-10-22T04:00:00.000Z");
});

test("loading, success, refresh error and retry preserve a correctly labelled snapshot", async () => {
  const state = new api.WebsiteTrafficState();
  let finish;
  const pending = state.load(
    query,
    () =>
      new Promise((resolve) => {
        finish = resolve;
      })
  );
  assert.equal(state.status, "loading");
  assert.equal(state.data, null);
  finish({ data: fixture });
  await pending;
  assert.equal(state.status, "success");
  assert.equal(state.data, fixture);
  assert.ok(state.updatedAt);
  await state.load({ ...query, domain: "new.example" }, async () => {
    throw Error("offline");
  });
  assert.equal(state.status, "error");
  assert.equal(state.data, fixture);
  assert.equal(state.query.domain, undefined);
  await state.load(query, async () => ({ data: { ...fixture, items: [] } }));
  assert.equal(state.status, "success");
  assert.equal(state.data.items.length, 0);
});

test("out of order replies and unmounted replies cannot replace latest state", async () => {
  const state = new api.WebsiteTrafficState();
  let finish;
  const pending = state.load(
    query,
    () =>
      new Promise((resolve) => {
        finish = resolve;
      })
  );
  await state.load({ ...query, domain: "latest.example" }, async () => ({
    data: fixture,
  }));
  finish({ data: { ...fixture, items: [] } });
  await pending;
  assert.equal(state.query.domain, "latest.example");
  assert.equal(state.data.items.length, 1);
  const cancelled = new api.WebsiteTrafficState();
  const waiting = cancelled.load(
    query,
    () =>
      new Promise((resolve) => {
        finish = resolve;
      })
  );
  cancelled.cancel();
  finish({ data: fixture });
  await waiting;
  assert.equal(cancelled.data, null);
});

test("东八区墙钟输入按 UTC+8 换算，不受浏览器时区影响", () => {
  const params = api.websiteTrafficParams({
    start: "2026-10-22T11:00",
    end: "2026-10-23T11:00",
  });
  assert.equal(params.start, "2026-10-22T03:00:00.000Z");
  assert.equal(params.end, "2026-10-23T03:00:00.000Z");
  assert.throws(
    () =>
      api.websiteTrafficParams({
        start: "2026-10-22T11:30",
        end: "2026-10-23T11:00",
      }),
    /整点/
  );
});

const source = readFileSync(
  new URL(
    "../../views/subcription/servers/WebsiteTraffic.vue",
    import.meta.url
  ),
  "utf8"
);
const { descriptor } = parse(source);
const compiled = compileScript(descriptor, {
  id: "website-traffic-test",
  inlineTemplate: true,
});
let snapshot = {};
const Slot = {
  setup:
    (_, { slots }) =>
    () =>
      h("div", slots.default?.()),
};
const component = evaluate(compiled.content, {
  "@/api/proxy/websiteTraffic": {
    ...api,
    WebsiteTrafficState: class extends api.WebsiteTrafficState {
      constructor() {
        super();
        Object.assign(this, snapshot);
      }
    },
  },
  "@/ui": {
    UiPanel: Slot,
    UiSkeleton: { render: () => h("div", "加载骨架") },
    UiButton: {
      setup:
        (_, { slots, attrs }) =>
        () =>
          h("button", attrs, slots.default?.()),
    },
  },
  "@/utils/traffic": { formatBytes: (value) => `${value} B` },
  "@/api/proxy": {
    listProxyServers: async () => ({ data: [] }),
  },
  "@/utils/datetime": datetime,
}).default;
async function render(state) {
  snapshot = state;
  return renderToString(createSSRApp(component, { servers: [] }));
}

test("view renders loading, empty, no-results, failure retry and success as read-only", async () => {
  assert.match(await render({ status: "loading" }), /加载骨架/);
  const empty = await render({
    status: "success",
    data: { ...fixture, items: [] },
    query: api.websiteTrafficParams(query),
    updatedAt: fixture.end,
  });
  assert.match(empty, /最近 24 小时暂无代理网站流量/);
  assert.match(await render({ status: "error" }), /重试/);
  const success = await render({
    status: "success",
    data: fixture,
    query: api.websiteTrafficParams(query),
    updatedAt: fixture.end,
  });
  assert.match(success, /测试节点/);
  assert.match(success, /未知（IP \/ 无法识别）/);
  assert.match(success, /42 B/);
  assert.match(success, /只读/);
  assert.match(success, /不回填历史/);
  const stale = await render({
    status: "error",
    data: fixture,
    query: api.websiteTrafficParams(query),
    updatedAt: fixture.end,
  });
  assert.match(stale, /当前显示的是上次查询结果/);
  assert.match(stale, /测试节点/);
});

test("默认收起，只渲染前 5 条并提供展开入口", async () => {
  const many = {
    ...fixture,
    items: Array.from({ length: 8 }, (_, i) => ({
      serverId: 3,
      serverName: "测试节点",
      domain: `site${i + 1}.example`,
      up: 1,
      down: 1,
      total: 2,
      rank: i + 1,
    })),
  };
  const html = await render({
    status: "success",
    data: many,
    query: api.websiteTrafficParams(query),
    updatedAt: fixture.end,
  });
  for (const visible of ["site1.example", "site5.example"])
    assert.match(html, new RegExp(visible));
  for (const hidden of ["site6.example", "site7.example", "site8.example"])
    assert.doesNotMatch(html, new RegExp(hidden));
  assert.match(html, /已收起 3 条/);
  assert.match(html, /展开全部 8 条/);
  assert.match(html, /aria-expanded="false"/);
});

test("offline retains old data, freshness and an explained disabled query action", async () => {
  browser.onLine = false;
  try {
    const html = await render({
      status: "success",
      data: fixture,
      query: api.websiteTrafficParams(query),
      updatedAt: fixture.end,
    });
    assert.match(html, /当前离线/);
    assert.match(html, /测试节点/);
    assert.match(html, /上次查询/);
    assert.match(html, /disabled/);
    assert.match(html, /联网后刷新/);
    const noCache = await render({});
    assert.match(noCache, /暂无缓存数据/);
  } finally {
    browser.onLine = true;
  }
});

test("website overview has no filter controls or extra server-list request", async () => {
  const html = await render({
    status: "success",
    data: fixture,
    updatedAt: fixture.end,
  });
  assert.doesNotMatch(html, /<(form|input|select)\b/);
  assert.doesNotMatch(html, /筛选|排序|排行条数|清空节点/);
  assert.match(html, /刷新/);
  assert.doesNotMatch(source, /listProxyServers/);
});

test("cancel restores a loadable state and ignores the late response", async () => {
  const state = new api.WebsiteTrafficState();
  let finish;
  const pending = state.load(query, () => new Promise(resolve => { finish = resolve; }));
  state.cancel();
  assert.equal(state.status, "idle");
  await state.load(query, async () => ({ data: fixture }));
  finish({ data: { ...fixture, items: [] } });
  await pending;
  assert.equal(state.data.items.length, 1);
  assert.equal(state.status, "success");
});

test("traffic share uses all traffic, including entries outside the displayed ranking", async () => {
  const html = await render({ status: "success", data: { ...fixture,
    summary: { up: 120, down: 300, total: 420 } }, updatedAt: fixture.end });
  assert.match(html, /10.0%/);
  assert.match(html, /总流量/);
  assert.match(html, /网站 \/ 服务器/);
});

test("refresh pauses when hidden, offline or deactivated and never overlaps", async () => {
  const hooks = {};
  const timers = new Map();
  const events = {};
  let timerId = 0, calls = 0, release;
  let pending = new Promise(resolve => { release = resolve; });
  const document = { visibilityState: "visible",
    addEventListener: (name, fn) => { events[name] = fn; },
    removeEventListener: name => { delete events[name]; } };
  const fakeWindow = { addEventListener: (name, fn) => { events[name] = fn; },
    removeEventListener: name => { delete events[name]; } };
  const vue = require("vue");
  let state;
  const lifecycleComponent = evaluate(compiled.content, {
    vue: { ...vue,
      onMounted: fn => { hooks.mount = fn; },
      onActivated: fn => { hooks.activate = fn; },
      onDeactivated: fn => { hooks.deactivate = fn; },
      onBeforeUnmount: fn => { hooks.unmount = fn; } },
    "@/api/proxy/websiteTraffic": { WebsiteTrafficState: class extends api.WebsiteTrafficState {
      constructor() { super(); state = this; }
      async load() { calls++; this.status = "loading"; await pending;
        this.status = "success"; this.updatedAt = new Date().toISOString(); }
    } },
    "@/ui": {}, "@/utils/traffic": { formatBytes: String }, "@/utils/datetime": datetime,
  }, { window: fakeWindow, document,
    setTimeout: fn => { const id = ++timerId; timers.set(id, fn); return id; },
    clearTimeout: id => timers.delete(id) }).default;
  lifecycleComponent.setup({}, { expose() {} });
  hooks.mount();
  hooks.activate();
  assert.equal(calls, 1);
  const tick = [...timers.values()][0]; timers.clear();
  await tick();
  assert.equal(calls, 1, "pending request must not overlap");
  release(); await Promise.resolve(); await Promise.resolve();
  document.visibilityState = "hidden"; events.visibilitychange();
  assert.equal(timers.size, 0);
  state.updatedAt = "";
  document.visibilityState = "visible"; events.visibilitychange();
  await Promise.resolve();
  assert.equal(calls, 2);
  browser.onLine = false; events.offline();
  assert.equal(timers.size, 0);
  browser.onLine = true; events.online();
  assert.equal(timers.size, 1);
  hooks.deactivate();
  assert.equal(timers.size, 0);
  events.visibilitychange();
  assert.equal(timers.size, 0);
  hooks.activate();
  assert.equal(timers.size, 1);
  hooks.unmount();
  assert.equal(timers.size, 0);
  assert.equal(Object.keys(events).length, 0);
});
