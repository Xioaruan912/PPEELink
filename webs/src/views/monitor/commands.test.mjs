import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

const exports = {};
vm.runInNewContext(
  ts.transpileModule(
    fs.readFileSync(new URL('./commands.ts', import.meta.url), 'utf8'),
    { compilerOptions: { module: ts.ModuleKind.CommonJS } }
  ).outputText,
  { exports }
);
const { quickCommands } = exports;

test('default commands diagnose servers without installations or destructive cleanup', () => {
  const defaults = quickCommands.filter((c) => !c.advanced);
  assert.ok(defaults.length >= 8);
  for (const c of defaults) {
    assert.ok(
      !/raw\.githubusercontent|prune|down --volumes|vacuum-time|autoremove/.test(c.command),
      c.name
    );
    assert.equal(c.source, 'builtin', c.name);
  }
  assert.ok(defaults.some((c) => c.name.includes('Agent')));
  assert.ok(defaults.some((c) => c.name.includes('Xray')));
  assert.ok(defaults.some((c) => c.name.includes('监听')));
  assert.ok(!quickCommands.some((c) => c.name === '重建当前 Compose'));
});

test('nodescripts catalog is categorized, advanced-gated and marks reinstalls dangerous', () => {
  const scripts = quickCommands.filter((c) => c.source === 'nodescripts');
  assert.ok(scripts.length >= 25, 'expected full NodeScripts catalog');
  const expectedCategories = new Set([
    '测速与检测',
    '性能基准',
    '网络与转发',
    '运维与面板',
    '系统重装',
  ]);
  const actualCategories = new Set(scripts.map((s) => s.category));
  for (const cat of expectedCategories) {
    assert.ok(actualCategories.has(cat), `missing category: ${cat}`);
  }
  for (const s of scripts) {
    assert.equal(s.advanced, true, `${s.name} must require confirmation`);
    assert.ok(s.command.trim().length > 0, `${s.name} must have a command`);
    if (s.category === '系统重装') {
      assert.equal(s.dangerous, true, `${s.name} must be marked dangerous`);
    }
  }
  assert.equal(quickCommands.filter((c) => c.name.includes('安装 Docker')).length, 1);
});

test('backup cleanup scripts are not an executable shortcut', () => {
  for (const c of quickCommands) {
    assert.ok(!c.command.includes('backup/backup.sh'), c.name);
  }
});
