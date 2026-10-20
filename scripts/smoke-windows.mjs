// Windows executable smoke test; isolated history and target, no developer tools in child PATH.
import assert from 'node:assert/strict'
import { spawn, execFileSync } from 'node:child_process'
import { copyFileSync, mkdtempSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import http from 'node:http'
import { once } from 'node:events'
import { setTimeout as delay } from 'node:timers/promises'
const root = fileURLToPath(new URL('../', import.meta.url))
assert.equal(process.platform, 'win32', 'This smoke script is verified for Windows only')
const dir = mkdtempSync(path.join(tmpdir(), 'storm-release-smoke-'))
const exe = path.join(dir, 'storm.exe')
copyFileSync(process.argv[2] ? path.resolve(process.argv[2]) : path.join(root, 'release/windows-amd64/storm.exe'), exe)
const env = { ...process.env, LOCALAPPDATA: dir, Path: '', PATH: '' }
const version=readFileSync(path.join(root,'backend/internal/buildinfo/VERSION'),'utf8').trim()
assert.equal(execFileSync(exe, ['--version'], { cwd: dir, env, encoding: 'utf8' }).trim(), 'Storm '+version)
let child, base
async function boot() {
 child = spawn(exe, ['--port', '0', '--no-browser'], { cwd: dir, env, windowsHide: true })
 let log = ''
 child.stdout.on('data', b => { log += b })
 child.stderr.on('data', b => { log += b })
 for (let n=0; n<100; n++) {
  const match = log.match(/running at (http:\/\/127\.0\.0\.1:\d+)/)
  if (match) { base=match[1]; return }
  if (child.exitCode !== null) throw new Error(log)
  await delay(100)
 }
 throw new Error('Startup timeout: '+log)
}
async function halt() { if (child && child.exitCode === null) { const exited=once(child,'exit'); child.kill(); await exited } }
async function api(route, body, expected=200) {
 const response=await fetch(base+route, body===undefined ? {} : {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
 assert.equal(response.status,expected,await response.clone().text())
 return response.json()
}
async function waitRun(id, predicate = r => r.persistence?.status === 'saved') {
 for(let n=0;n<150;n++){const r=await api('/api/tests/'+id);if(predicate(r))return r;await delay(100)}
 throw new Error('Run timeout '+id)
}
function accounting(s, intervals) {
 assert.equal(s.requests,s.started_requests)
 assert.equal(s.requests,s.successes+s.failures)
 assert.equal(s.failures,s.http_error_count+s.network_errors+s.canceled)
 assert.equal(s.scheduled_requests,s.dispatched_requests+s.missed_schedules)
 assert.ok(s.started_requests<=s.dispatched_requests)
 assert.equal(intervals.reduce((n,p)=>n+p.completed,0),s.requests)
}
const target=http.createServer((req,res)=>{
 if(req.url==='/slow'){const timer=setTimeout(()=>res.end('ok'),1000);res.on('close',()=>clearTimeout(timer))}
 else {res.setHeader('Content-Type','application/json');res.end('{"ok":true}')}
})
target.listen(0,'127.0.0.1');await once(target,'listening')
const targetURL=`http://127.0.0.1:${target.address().port}`
const config={url:targetURL+'/fast',method:'GET',workers:4,rps:40,duration_seconds:3,authorized:true}
try {
 await boot()
 assert.equal((await api('/api/health')).version,version)
 assert.equal((await fetch(base+'/')).status,200)
 assert.match(await (await fetch(base+'/history/test',{headers:{Accept:'text/html'}})).text(),/<!doctype html>/i)
 assert.equal((await fetch(base+'/missing.js')).status,404)
 assert.equal((await fetch(base+'/api/missing')).headers.get('Content-Type'),'application/json')
 for(const method of ['POST','DELETE'])assert.equal((await fetch(base+'/api/tests',{method,headers:{Origin:'https://evil.example','Content-Type':'application/json'},body:'{}'})).status,403)
 // Fetch may replace Host with the URL authority; use raw HTTP for this check.
 const hostStatus=await new Promise((resolve,reject)=>{
  const req=http.get(base+'/api/health',{headers:{Host:'evil.example'}},res=>{res.resume();resolve(res.statusCode)})
  req.on('error',reject)
 })
 assert.equal(hostStatus,403)
 const a=await api('/api/tests',config,202)
 await api('/api/tests',config,409)
 const final=await waitRun(a.id);assert.equal(final.status,'completed');accounting(final.summary,final.intervals)
 const stored=await api('/api/history/'+a.id)
 const b=await api('/api/tests',{...config,mode:'stepped',rps:0,duration_seconds:0,start_rps:40,rps_increment:20,max_rps:60,stage_duration_seconds:3,thresholds:{max_p95_latency_ms:200,max_error_rate_percent:1,min_start_rate_percent:95}},202)
 const stages=await waitRun(b.id);assert.equal(stages.status,'completed');assert.equal(stages.stages.length,2);assert.ok(stages.analysis);for(const s of stages.stages)accounting(s.summary,s.intervals)
 const c=await api('/api/tests',{...config,url:targetURL+'/slow',duration_seconds:30},202)
 await waitRun(c.id,r=>r.progress.started_requests>0)
 await api('/api/tests/'+c.id+'/stop',{},202)
 const canceled=await waitRun(c.id);assert.equal(canceled.status,'canceled');accounting(canceled.summary,canceled.intervals)
 // Restart, then force termination during active execution to exercise recovery.
 await halt();await boot();assert.deepEqual(await api('/api/history/'+a.id),stored)
 const d=await api('/api/tests',{...config,duration_seconds:30},202)
 await waitRun(d.id,r=>r.status==='running' && r.progress.started_requests>0)
 await halt();await boot()
 assert.equal((await api('/api/history/'+d.id)).status,'interrupted')
 assert.equal((await api('/api/tests/active')).run,null)
 assert.deepEqual(await api('/api/history/'+a.id),stored)
 console.log(JSON.stringify({result:'PASS',directory:dir,standard:a.id,stepped:b.id,canceled:c.id,interrupted:d.id,breakpoint:stages.analysis.breakpoint.status},null,2))
} finally { await halt();target.closeAllConnections();await new Promise(resolve=>target.close(resolve)) }
