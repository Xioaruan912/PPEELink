import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';
function harness(){
 const exports={};const timers=[];const sockets=[];let tickets=0;
 class Socket{static OPEN=1;readyState=1;sent=[];constructor(){sockets.push(this)}send(x){this.sent.push(JSON.parse(x))}close(){this.closed=true}}
 vm.runInNewContext(ts.transpileModule(fs.readFileSync(new URL('./terminalConnection.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports,WebSocket:Socket,location:{protocol:'https:',host:'test.invalid'},Uint8Array,atob:x=>Buffer.from(x,'base64').toString('binary'),setTimeout:fn=>{timers.push(fn);return timers.length},clearTimeout:id=>{if(id)timers[id-1]=null}});
 const events=[];const c=exports.createTerminalConnection({ticket:async()=>{tickets++;return 'ticket'},event:e=>events.push(e)});
 return {c,sockets,timers,events,tickets:()=>tickets};
}
test('reconnects the same session without replaying input and stops on leave',async()=>{
 const h=harness();await h.c.connect(1,'a'.repeat(24));const first=h.sockets[0];first.onopen();
 first.onmessage({data:JSON.stringify({type:'ready',sessionId:'a'.repeat(24),persistent:true})});
 h.c.input('echo once\r');assert.equal(first.sent.length,1);
 first.onclose();const retry=h.timers.filter(Boolean);assert.equal(retry.length,1);await retry[0]();
 assert.equal(h.tickets(),2);assert.equal(h.sockets[1].sent.length,0);
 h.c.leave();assert.equal(h.sockets[1].closed,true);assert.equal(h.sockets[1].onclose,null);
});
test('exit and authorization revocation never retry or recreate a session',async()=>{
 for(const type of ['exit','unauthorized']){const h=harness();await h.c.connect(1,'b'.repeat(24));h.sockets[0].onmessage({data:JSON.stringify({type})});assert.equal(h.timers.filter(Boolean).length,0);assert.equal(h.events.at(-1).type,type)}
});
test('repeated connects do not create duplicate sockets and leaving invalidates tickets',async()=>{
 const h=harness();await Promise.all([h.c.connect(1,'a'.repeat(24)),h.c.connect(1,'a'.repeat(24))]);assert.equal(h.sockets.length,1);
 const events=[];let resolve;const exports={};vm.runInNewContext(ts.transpileModule(fs.readFileSync(new URL('./terminalConnection.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText,{exports,WebSocket:class{constructor(){throw Error('stale ticket opened socket')}},location:{protocol:'https:',host:'x'},setTimeout,clearTimeout});
 const c=exports.createTerminalConnection({ticket:()=>new Promise(r=>resolve=r),event:e=>events.push(e)});const pending=c.connect(1,'a'.repeat(24));c.leave();resolve('late');await pending;
});
