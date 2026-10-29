import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import ts from "typescript";
const exports = {};
vm.runInNewContext(
  ts.transpileModule(
    readFileSync(new URL("./network-throughput.ts", import.meta.url), "utf8"),
    { compilerOptions: { module: ts.ModuleKind.CommonJS } }
  ).outputText,
  { exports }
);
test("traffic budget accounts for sequential directions and shared bidirectional cap", () => {
  assert.equal(exports.estimatedTraffic(100, 10, 3, "both"), 750000000);
  assert.equal(exports.estimatedTraffic(100, 10, 3, "bidir"), 375000000);
  assert.equal(exports.estimatedTraffic(100, 10, 3, "forward"), 375000000);
});
test("curve normalizes real intervals and preserves zero samples", () => {
  assert.equal(exports.curvePoints([]), "");
  const points = exports.curvePoints([
    { end: 1, bitsPerSecond: 0 },
    { end: 2, bitsPerSecond: 100 },
  ]);
  assert.equal(points, "0,70 280,0");
});
