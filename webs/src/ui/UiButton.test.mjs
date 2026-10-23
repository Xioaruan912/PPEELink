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
const { descriptor } = parse(
  readFileSync(new URL("./UiButton.vue", import.meta.url), "utf8")
);
const { content } = compileScript(descriptor, {
  id: "button-test",
  inlineTemplate: true,
});
const exports = {};
vm.runInNewContext(
  ts.transpileModule(content, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText,
  {
    exports,
    require: (name) =>
      name === "@/motion" ? { vMagnetic: {} } : require(name),
  }
);
test("small buttons consume size instead of leaking an UnoCSS 24rem attribute", async () => {
  const html = await renderToString(
    createSSRApp({
      render: () => h(exports.default, { size: "sm" }, () => "升级 Agent"),
    })
  );
  assert.doesNotMatch(html, /\bsize="sm"/);
  assert.match(html, /ui-button--sm/);
  assert.match(html, /升级 Agent/);
});
