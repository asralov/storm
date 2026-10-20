import { execFileSync } from 'node:child_process'
import { cpSync, mkdirSync, rmSync, readFileSync, writeFileSync, renameSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { packageRelease } from './package-release.mjs'
const root = fileURLToPath(new URL('../', import.meta.url))
const frontend = path.join(root, 'frontend'), backend = path.join(root, 'backend')
const embedded = path.join(backend, 'internal/webui/dist')
const os = process.env.GOOS || 'windows', arch = process.env.GOARCH || 'amd64'
if (!['windows', 'linux', 'darwin'].includes(os) || !['amd64', 'arm64'].includes(arch)) throw new Error('Unsupported build target')
const version = readFileSync(path.join(backend,'internal/buildinfo/VERSION'),'utf8').trim()
if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)) throw new Error('Invalid VERSION')
if (process.env.STORM_VERSION && process.env.STORM_VERSION !== version) throw new Error('Edit the canonical VERSION file; STORM_VERSION must not override it')
const run = (cmd, args, cwd, env = process.env) => execFileSync(cmd, args, { cwd, env, stdio: 'inherit' })
const npm = args => process.platform === 'win32'
 ? run('cmd.exe', ['/d', '/s', '/c', `npm ${args.join(' ')}`], frontend)
 : run('npm', args, frontend)
// Fixed repository-contained generated path; discard stale assets before building.
rmSync(embedded, { recursive: true, force: true })
npm(['ci']); npm(['run', 'build'])
cpSync(path.join(frontend, 'dist'), embedded, { recursive: true })
const outputDir=path.join(root,'release',`${os}-${arch}`)
mkdirSync(outputDir,{recursive:true})
const output=path.join(outputDir,`storm${os==='windows'?'.exe':''}`)
const pending=output+'.pending'
run('go',['build','-trimpath','-tags=production','-ldflags','-s -w','-o',pending,'./cmd/storm'],backend,{...process.env,CGO_ENABLED:'0',GOOS:os,GOARCH:arch})
renameSync(pending,output)
const capture=(cmd,args,cwd=root)=>execFileSync(cmd,args,{cwd,encoding:'utf8'}).trim()
const metadata={version,target:`${os}/${arch}`,cgo:false,commit:capture('git',['rev-parse','HEAD']),dirty:Boolean(capture('git',['status','--porcelain'])),go:capture('go',['version'],backend),node:process.version,built_at:new Date().toISOString()}
writeFileSync(path.join(outputDir,'build-metadata.json'),JSON.stringify(metadata,null,2)+'\n')
if(os==='windows' && arch==='amd64') packageRelease({root,backend,frontend,outputDir,version})
console.log(`Built ${output}`)
