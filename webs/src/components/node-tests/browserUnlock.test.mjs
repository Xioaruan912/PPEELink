import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import ts from "typescript";
function load(timer = setTimeout) {
  const exports = {};
  vm.runInNewContext(
    ts.transpileModule(
      readFileSync(new URL("./browserUnlock.ts", import.meta.url), "utf8"),
      {
        compilerOptions: {
          module: ts.ModuleKind.CommonJS,
          target: ts.ScriptTarget.ES2022,
        },
      }
    ).outputText,
    {
      exports,
      AbortController,
      DOMException,
      TextDecoder,
      setTimeout: timer,
      clearTimeout,
      Date,
      fetch,
    }
  );
  return exports;
}
const svc = (key, kind = "trace") => ({
  key,
  name: key,
  group: "ai",
  kind,
  url: "https://example.test/probe",
  allow: "US JP",
});
test("API errors and unknown shapes are not confirmed unlock", () => {
  const { classifyBrowserResponse: read } = load();
  for (const [code, text] of [
    [429, '{"error":{}}'],
    [200, '{"anything":true}'],
    [403, "<html>Just a moment</html>"],
    [200, '{"required":null}'],
  ]) {
    const r = read(svc("openai", "openai"), code, text);
    assert.equal(r.status, "unknown");
    assert.equal(r.ok, false);
  }
  assert.equal(
    read(svc("openai", "openai"), 200, '{"required":false}').status,
    "available"
  );
  assert.equal(
    read(
      svc("openai", "openai"),
      403,
      '{"error":{"code":"unsupported_country"}}'
    ).status,
    "blocked"
  );
});
test("only the target service region is used, with inference preserved", () => {
  const { classifyBrowserResponse: read } = load();
  const r = read(svc("claude"), 200, "loc=JP\nip=203.0.113.2\n");
  assert.equal(r.status, "inferred");
  assert.equal(r.evidence, "region");
  assert.equal(r.region, "JP");
  assert.equal(r.ok, false);
  assert.equal(read(svc("claude"), 200, "loc=CN\n").status, "region_blocked");
  assert.equal(read(svc("claude"), 403, "loc=US\n").status, "unknown");
});
test("opaque browser responses are reachability only", async () => {
  const { runBrowserUnlock } = load();
  let count = 0;
  const r = await runBrowserUnlock(
    [svc("claude")],
    new AbortController().signal,
    () => {},
    async (url, init) => {
      count++;
      if (init.mode === "cors") throw new TypeError("CORS");
      return { type: "opaque", status: 0, body: null };
    }
  );
  assert.equal(count, 2);
  assert.equal(r.source, "browser");
  assert.equal(r.results[0].status, "reachable");
  assert.equal(r.results[0].evidence, "reach");
  assert.equal(r.ok, false);
});
test("runner caps concurrency at four and abort stops queue and fallbacks", async () => {
  const { runBrowserUnlock } = load();
  let active = 0,
    peak = 0,
    calls = 0,
    updates = 0;
  const controller = new AbortController();
  const fetcher = async (url, { signal }) => {
    calls++;
    peak = Math.max(peak, ++active);
    try {
      await new Promise((resolve, reject) => {
        signal.addEventListener(
          "abort",
          () => reject(new DOMException("stopped", "AbortError")),
          { once: true }
        );
      });
    } finally {
      active--;
    }
  };
  const pending = runBrowserUnlock(
    Array.from({ length: 12 }, (_, i) => svc(String(i))),
    controller.signal,
    () => updates++,
    fetcher
  );
  setTimeout(() => controller.abort(), 15);
  const r = await pending;
  assert.equal(peak, 4);
  assert.equal(calls, 4);
  assert.equal(active, 0);
  assert.equal(updates, 0);
  assert.equal(r.complete, false);
  assert.equal(r.results.length, 12);
  assert.ok(r.results.every((x) => x.status === "unknown"));
});
test("whole-run deadline aborts slow requests and leaves explicit incomplete result", async () => {
  const { runBrowserUnlock } = load((fn, ms) =>
    setTimeout(fn, ms === 45000 ? 15 : ms)
  );
  const r = await runBrowserUnlock(
    [svc("claude")],
    new AbortController().signal,
    () => {},
    async (url, { signal }) => {
      await new Promise((resolve, reject) =>
        signal.addEventListener(
          "abort",
          () => reject(new DOMException("timeout", "AbortError")),
          { once: true }
        )
      );
    }
  );
  assert.equal(r.complete, false);
  assert.match(r.error, /超时/);
  assert.equal(r.results[0].status, "unknown");
});
test("result labels never call region inference or reachable responses unlocked", () => {
  const { unlockLabel, unlockState } = load();
  assert.equal(
    unlockLabel({ status: "inferred", ok: false, evidence: "region" }),
    "推测可用"
  );
  assert.equal(
    unlockLabel({ status: "reachable", ok: false, evidence: "reach" }),
    "仅可访问"
  );
  assert.equal(
    unlockLabel({ status: "unknown", ok: false, evidence: "reach" }),
    "无法判断"
  );
  assert.equal(
    unlockState({ status: "inferred", ok: false, evidence: "region" }),
    "warn"
  );
});

test("legacy available results cannot claim API confirmation", () => {
  const { unlockLabel, unlockState } = load();
  assert.equal(
    unlockLabel({ status: "available", ok: true }),
    "历史结果，请重新检测"
  );
  assert.equal(unlockState({ status: "available", ok: true }), "warn");
});
