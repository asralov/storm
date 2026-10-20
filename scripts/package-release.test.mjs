import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { collectNpmNotices } from './package-release.mjs'

function fixture(t, { version='1.0.0', installedVersion=version, name='example', missing=false, license='MIT test fixture', nested=false, optional=false }={}) {
 const root=mkdtempSync(path.join(tmpdir(),'storm-notice-test-'))
 t.after(()=>{
  assert.equal(path.dirname(root),path.resolve(tmpdir()))
  assert.ok(path.basename(root).startsWith('storm-notice-test-'))
  rmSync(root,{recursive:true,force:true})
 })
 writeFileSync(path.join(root,'package-lock.json'),JSON.stringify({packages:{
  '':{name:'frontend'},
  [`node_modules/${name}`]:{version,license:'MIT',optional},
  'node_modules/dev-only':{version:'1.0.0',dev:true},
 }}))
 if(!missing) {
  const dir=path.join(root,'node_modules',name)
  mkdirSync(dir,{recursive:true})
  writeFileSync(path.join(dir,'package.json'),JSON.stringify({name,version:installedVersion}))
  if(license!==null) {
   const target=nested?path.join(dir,'vendor','submodule'):dir
   mkdirSync(target,{recursive:true})
   writeFileSync(path.join(target,'LICENSE'),license)
  }
 }
 return root
}

test('collects nested notices and skips build-only packages',t=>{
 const result=collectNpmNotices(fixture(t,{nested:true}))
 assert.equal(result.length,1)
 assert.match(result[0].label,/example 1.0.0/)
 assert.equal(result[0].files.length,1)
})
test('missing production directory fails instead of omitting notices',t=>{
 assert.throws(()=>collectNpmNotices(fixture(t,{missing:true})),/Missing production dependency/)
})
test('optional production omissions require explicit review too',t=>{
 assert.throws(()=>collectNpmNotices(fixture(t,{missing:true,optional:true})),/Missing production dependency/)
})
test('installed version must match lockfile',t=>{
 assert.throws(()=>collectNpmNotices(fixture(t,{installedVersion:'2.0.0'})),/differs from lockfile/)
})
test('missing and empty licenses fail',t=>{
 for(const license of [null,' '])assert.throws(()=>collectNpmNotices(fixture(t,{license})),/license text/)
})
test('invalid UTF-8 and replacement characters fail',t=>{
 for(const license of [Buffer.from([0x97]),'Copyright \uFFFD'])assert.throws(()=>collectNpmNotices(fixture(t,{license})))
})
test('Victory upgrade requires supplemental notice review',t=>{
 assert.throws(()=>collectNpmNotices(fixture(t,{name:'victory-vendor',version:'38.0.0'})),/Victory version changed/)
})
test('reviewed Victory version is accepted',t=>{
 assert.equal(collectNpmNotices(fixture(t,{name:'victory-vendor',version:'37.3.6'})).length,1)
})
