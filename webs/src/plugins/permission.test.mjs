import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import ts from "typescript";
const exports = {};
vm.runInNewContext(ts.transpileModule(
  readFileSync(new URL("./permission.ts", import.meta.url), "utf8") + "\nexport { landingFor };",
  { compilerOptions: { module: ts.ModuleKind.CommonJS } }
).outputText, { exports, require: () => ({}) });
const { landingFor } = exports;
test("home opens subscriptions for each role and keeps password change mandatory", () => {
  const admin = { roles: ["ADMIN"] }, user = { roles: ["USER"] };
  assert.equal(landingFor(admin, "/"), "/subcription/subs");
  assert.equal(landingFor(user, "/"), "/my/subscriptions");
  assert.equal(landingFor(user, "/subcription/subs"), "/my/subscriptions");
  assert.equal(landingFor(user, "/dashboard"), "/my/subscriptions");
  assert.equal(landingFor(admin, "/dashboard"), "");
  assert.equal(landingFor(admin, "/subcription/subs"), "");
  assert.equal(landingFor({ ...admin, mustChangePassword: true }, "/subcription/subs"), "/system/user/set");
  assert.equal(landingFor({ ...user, mustChangePassword: true }, "/my/subscriptions"), "/system/user/set");
  assert.equal(landingFor({ ...user, mustChangePassword: true }, "/system/user/set"), "");
});
