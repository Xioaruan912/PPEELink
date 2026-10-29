import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
import ts from "typescript";
const handlers = [];
const service = {
  interceptors: {
    request: { use() {} },
    response: {
      use(ok, fail) {
        handlers.push(fail);
      },
    },
  },
};
const source = fs
  .readFileSync(new URL("./request.ts", import.meta.url), "utf8")
  .replace(/^import .*;$/gm, "")
  .replace("import.meta.env.VITE_APP_BASE_API", "''");
vm.runInNewContext(
  ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText,
  {
    exports: {},
    axios: { create: () => service },
    ElMessage: { error() {} },
    ElMessageBox: { confirm: () => Promise.resolve() },
    useUserStoreHook: () => ({}),
    ArrayBuffer,
    Blob,
  }
);
test("diagnostic validation retains the server reason and cancellation identity", async () => {
  const fail = handlers[0];
  const err = {
    message: "Request failed with status code 400",
    response: { data: { code: "40000", msg: "中转关系存在循环" } },
  };
  await assert.rejects(fail(err), (e) => e.message === "中转关系存在循环");
  const cancel = { message: "canceled", code: "ERR_CANCELED" };
  await assert.rejects(fail(cancel), (e) => e.code === "ERR_CANCELED");
});
