import test from 'node:test';import assert from 'node:assert/strict';import fs from 'node:fs';import vm from 'node:vm';import ts from 'typescript';
const exports={};vm.runInNewContext(ts.transpileModule(fs.readFileSync(new URL('./commands.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText,{exports});
const {quickCommands}=exports;
test('default commands diagnose servers without installations or destructive cleanup',()=>{
 const defaults=quickCommands.filter(c=>!c.advanced);assert.ok(defaults.length>=8);
 for(const c of defaults){assert.ok(!/raw\.githubusercontent|prune|down --volumes|vacuum-time|autoremove/.test(c.command),c.name)}
 assert.ok(defaults.some(c=>c.name.includes('Agent')));assert.ok(defaults.some(c=>c.name.includes('Xray')));assert.ok(defaults.some(c=>c.name.includes('监听')));
 assert.ok(!quickCommands.some(c=>c.name==='重建当前 Compose'));
});
test('external scripts are pinned, checked and kept in advanced tools',()=>{
 const commands=quickCommands.flatMap(c=>[c,...(c.variants||[])]);
 for(const c of commands.filter(c=>c.command.includes('raw.githubusercontent.com'))){
  assert.match(c.command,/Xcript\/[a-f0-9]{40}\//);assert.match(c.command,/sha256sum -c -/);assert.match(c.command,/curl -f/);assert.ok(!c.command.includes('bash <('));
 }
 assert.equal(quickCommands.filter(c=>c.name.includes('安装 Docker')).length,1);
});

test('backup cleanup scripts are not an executable shortcut',()=>{for(const c of quickCommands)assert.ok(!c.command.includes('backup/backup.sh'),c.name)});
