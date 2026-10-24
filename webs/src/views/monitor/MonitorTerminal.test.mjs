import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import { createRequire } from "node:module";
import ts from "typescript";
import { parse, compileScript } from "vue/compiler-sfc";
const require = createRequire(import.meta.url);
function harness() {
  const { descriptor } = parse(
    readFileSync(new URL("./MonitorTerminal.vue", import.meta.url), "utf8")
  );
  const { content } = compileScript(descriptor, { id: "terminal-test" });
  let resolveTicket;
  let requests = 0;
  const sockets = [];
  class Socket {
    static OPEN = 1;
    readyState = 1;
    sent = [];
    constructor() {
      sockets.push(this);
    }
    send(data) {
      this.sent.push(JSON.parse(data));
    }
    close() {
      this.closed = true;
    }
  }
  const exports = {};
  const vue = require("vue");
  vm.runInNewContext(
    ts.transpileModule(content, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
      },
    }).outputText,
    {
      exports,
      WebSocket: Socket,
      window: { innerWidth: 390, innerHeight: 760 },
      location: { protocol: "https:", host: "test.invalid" },
      require(name) {
        if (name === "vue") return { ...vue, watch() {}, onBeforeUnmount() {} };
        if (name === "@/api/monitor")
          return {
            getMonitorSSHTicket() {
              requests++;
              return new Promise((resolve) => {
                resolveTicket = resolve;
              });
            },
          };
        if (name === "element-plus")
          return { ElMessage: { warning() {} }, ElMessageBox: {} };
        if (name === "./commands") return { quickCommands: [] };
        return {};
      },
    }
  );
  const props = {
    modelValue: true,
    monitor: { id: 1, name: "Test", proxyServerId: 1 },
  };
  const state = exports.default.setup(props, { emit() {}, expose() {} });
  return {
    state,
    sockets,
    props,
    resolve() {
      resolveTicket({ data: { ticket: "test-ticket" } });
    },
    requests: () => requests,
  };
}
test("closing while ticket is pending does not open a background connection", async () => {
  const h = harness();
  const pending = h.state.connect();
  h.state.disconnect();
  h.props.modelValue = false;
  h.resolve();
  await pending;
  assert.equal(h.sockets.length, 0);
});
test("repeated connect clicks request only one ticket", async () => {
  const h = harness();
  const pending = h.state.connect();
  await h.state.connect();
  assert.equal(h.requests(), 1);
  h.resolve();
  await pending;
  assert.equal(h.sockets.length, 1);
});
test("touch keys send control sequences without appending Enter", async () => {
  const h = harness();
  const pending = h.state.connect();
  h.resolve();
  await pending;
  h.sockets[0].onopen();
  h.state.pressKey("\x03");
  h.state.pressKey("\x1b[A");
  assert.equal(h.sockets[0].sent[0].data, "\x03");
  assert.equal(h.sockets[0].sent[1].data, "\x1b[A");
  h.state.disconnect();
  h.state.pressKey("ignored");
  assert.equal(h.sockets[0].sent.length, 2);
  assert.equal(h.sockets[0].onmessage, null);
  assert.equal(h.sockets[0].closed, true);
});
