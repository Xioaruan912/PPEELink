import test from "node:test";
import assert from "node:assert/strict";
import vm from "node:vm";
import { readFileSync } from "node:fs";
import ts from "typescript";
function transpile(file, context) {
  const exports = {};
  vm.runInNewContext(
    ts.transpileModule(
      readFileSync(new URL(file, import.meta.url), "utf8").replaceAll(
        "import.meta.env.VITE_APP_BASE_API",
        '""'
      ),
      {
        compilerOptions: {
          module: ts.ModuleKind.CommonJS,
          target: ts.ScriptTarget.ES2022,
        },
      }
    ).outputText,
    { exports, ...context }
  );
  return exports;
}
function load(fetcher) {
  const globals = {
    TextDecoder,
    URLSearchParams,
    AbortController,
    setTimeout,
    clearTimeout,
    DOMException,
    localStorage: { getItem: () => "Bearer fixture" },
  };
  const session = transpile("./useNodeTestSession.ts", {
    ...globals,
    require: () => ({}),
  });
  return transpile("./nodeUnlock.ts", {
    ...globals,
    fetch: fetcher,
    require: () => session,
  });
}
const row = {
  key: "openai",
  name: "ChatGPT",
  group: "ai",
  status: "unknown",
  evidence: "api",
  ok: false,
  note: "unknown",
};
const final = {
  nodeName: "Fixture",
  source: "node",
  complete: true,
  ok: false,
  results: [row],
};
test("node stream returns incremental rows and requires completion", async () => {
  const rows = [];
  const { requestNodeUnlock } = load(async (url, init) => {
    assert.equal(init.headers.Accept, "text/event-stream");
    assert.equal(init.headers.Authorization, "Bearer fixture");
    assert.equal(init.body.get("group"), "ai");
    return new Response(
      `event: item\ndata: ${JSON.stringify(row)}\n\nevent: done\ndata: ${JSON.stringify(final)}\n\n`,
      { headers: { "Content-Type": "text/event-stream" } }
    );
  });
  const r = await requestNodeUnlock(
    1,
    "ai",
    new AbortController().signal,
    (r) => rows.push(r)
  );
  assert.equal(rows.length, 1);
  assert.equal(r.complete, true);
  assert.equal(r.results.length, 1);
});
test("truncated stream is incomplete even after some successful rows", async () => {
  const { requestNodeUnlock } = load(
    async () =>
      new Response(`event:item\ndata:${JSON.stringify(row)}\n\n`, {
        headers: { "Content-Type": "text/event-stream" },
      })
  );
  await assert.rejects(
    requestNodeUnlock(1, "", new AbortController().signal, () => {}),
    /连接中断/
  );
});
test("legacy JSON response remains compatible and errors are surfaced", async () => {
  const { requestNodeUnlock } = load(async () =>
    Response.json({ code: "00000", data: final })
  );
  assert.equal(
    (await requestNodeUnlock(1, "", new AbortController().signal, () => {}))
      .source,
    "node"
  );
  const error = load(async () =>
    Response.json({ code: "42900", msg: "已有测试进行中" }, { status: 429 })
  );
  await assert.rejects(
    error.requestNodeUnlock(1, "", new AbortController().signal, () => {}),
    /已有测试进行中/
  );
});
