import test from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import ts from "typescript";
import vm from "node:vm";
const exports={};vm.runInNewContext(ts.transpileModule(readFileSync(new URL("./diagnosticBatch.ts",import.meta.url),"utf8"),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText,{exports});
const {runDiagnosticQueue,diagnosticVerdict}=exports;
test("batch stays at two requests and records errors without losing remaining nodes",async()=>{
 let active=0,max=0;const completed=[],errors=[];
 await runDiagnosticQueue([1,2,3,4],async n=>{active++;max=Math.max(max,active);await new Promise(r=>setTimeout(r,5));active--;if(n===2)throw Error("request failure");return n;},{signal:new AbortController().signal,onStart(){},onResult(n){completed.push(n)},onError(n){errors.push(n)},onCancel(){}});
 assert.equal(max,2);assert.deepEqual(completed.sort(),[1,3,4]);assert.deepEqual(errors,[2]);
});
test("cancel aborts scheduling and marks pending rows",async()=>{const c=new AbortController();const started=[],cancelled=[];await runDiagnosticQueue([1,2,3,4],async n=>{c.abort();return n;},{signal:c.signal,onStart(n){started.push(n)},onResult(){},onError(){},onCancel(n){cancelled.push(n)}});assert.deepEqual(started,[1]);assert.deepEqual(cancelled,[1,2,3,4]);});
test("HTTPS success proves connectivity despite an inconclusive UDP precheck",()=>{assert.equal(diagnosticVerdict({steps:[{key:"transport",status:"unknown"},{key:"https",status:"passed"}]}),"passed");assert.equal(diagnosticVerdict({steps:[{key:"dns",status:"failed"}]}),"failed");assert.equal(diagnosticVerdict({steps:[{key:"tunnel",status:"unknown"}]}),"unknown");});

test("finished rows can open details even when no protocol report was returned",()=>{const {canOpenDiagnosticDetails}=exports;for(const status of ["passed","failed","unknown","error","skipped","cancelled"]){assert.equal(canOpenDiagnosticDetails({status}),true,status)}for(const status of ["waiting","running"]){assert.equal(canOpenDiagnosticDetails({status}),false,status)}});
