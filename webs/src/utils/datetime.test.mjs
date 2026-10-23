// node --test src/utils/datetime.test.mjs
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import vm from "node:vm";
import ts from "typescript";

const require = createRequire(import.meta.url);
function evaluate(source, imports = {}) {
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
    Number,
    String,
    Math,
    console,
  });
  return exports;
}
const api = evaluate(readFileSync(new URL("./datetime.ts", import.meta.url), "utf8"));
const date = evaluate(readFileSync(new URL("./date.ts", import.meta.url), "utf8"), {
  "./datetime": api,
});

test("墙钟时间按东八区解析，与浏览器时区无关", () => {
  // 东八区 2026-10-22 19:00 == UTC 2026-10-22 11:00
  assert.equal(api.parseInstant("2026-10-22T19:00"), Date.UTC(2026, 9, 22, 11, 0));
  assert.equal(api.parseInstant("2026-10-22T19:00:30"), Date.UTC(2026, 9, 22, 11, 0, 30));
  // 带时区的时间保持绝对值不变
  assert.equal(api.parseInstant("2026-10-22T11:00:00Z"), Date.UTC(2026, 9, 22, 11, 0));
  assert.equal(api.parseInstant("2026-10-23T09:22:57.97751871+08:00"), Date.UTC(2026, 9, 23, 1, 22, 57, 977));
  // Go 本地时间字符串（无时区标记）同样按东八区解释
  assert.equal(api.parseInstant("2026-10-23 09:22:57"), Date.UTC(2026, 9, 23, 1, 22, 57));
  assert.equal(api.parseInstant(null), null);
  assert.equal(api.parseInstant(""), null);
  assert.equal(api.parseInstant("not-a-date"), null);
});

test("展示统一为东八区，日期不会错一天", () => {
  const late = "2026-10-23T02:30:00Z"; // 东八区 10-23 10:30
  assert.equal(api.formatDay(late), "2026-10-23");
  assert.equal(api.formatClock(late), "10:30");
  assert.equal(date.formatDateTime(late), "2026-10-23 10:30");
  // UTC 深夜在东八区已是次日：越界换日必须正确
  assert.equal(date.formatDateTime("2026-10-22T16:05:00Z"), "2026-10-23 00:05");
  assert.equal(api.formatDay("2026-10-22T16:05:00Z"), "2026-10-23");
  assert.equal(date.formatDateTime(""), "");
  assert.equal(date.formatDateTime("bad"), "");
});

test("东八区日期轴与今天一致，30 天窗口从今天倒数", () => {
  const now = Date.UTC(2026, 9, 23, 2, 46); // 东八区 10-23 10:46
  const keys = api.recentDayKeys(30, now);
  assert.equal(keys.length, 30);
  assert.equal(keys[0], "2026-09-24");
  assert.equal(keys[29], "2026-10-23");
  assert.equal(new Set(keys).size, 30);
  // 同一天不同时段生成同一条轴
  assert.deepEqual(api.recentDayKeys(30, Date.UTC(2026, 9, 23, 15, 30)), keys);
  assert.equal(api.dayStart(now), Date.UTC(2026, 9, 22, 16, 0));
});

test("默认查询窗口覆盖当前小时，便于刚开启采集就看到数据", () => {
  const now = Date.UTC(2026, 9, 23, 2, 46);
  const { start, end } = api.recentHourWindow(now);
  assert.equal(end, Date.UTC(2026, 9, 23, 3, 0));
  assert.equal(start, Date.UTC(2026, 9, 22, 3, 0));
  assert.equal(end - start, 24 * 3600000);
  for (const edge of [Date.UTC(2026, 9, 23, 2, 0), Date.UTC(2026, 9, 23, 2, 59, 59, 999)])
    assert.equal(api.recentHourWindow(edge).end, Date.UTC(2026, 9, 23, 3, 0));
  assert.equal(api.hourFloor(now), Date.UTC(2026, 9, 23, 2, 0));
});

test("输入框往返保持东八区墙钟值", () => {
  const { start, end } = api.recentHourWindow(Date.UTC(2026, 9, 23, 2, 46));
  assert.equal(api.toInputValue(start), "2026-10-22T11:00");
  assert.equal(api.toInputValue(end), "2026-10-23T11:00");
  assert.equal(api.fromInputValue(api.toInputValue(end)), end);
  assert.equal(api.fromInputValue("2026-10-23T11:00") - api.fromInputValue("2026-10-22T11:00"), 86400000);
});
