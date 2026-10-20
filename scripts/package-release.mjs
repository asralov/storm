import { execFileSync } from 'node:child_process'
import { readdirSync, readFileSync, existsSync, writeFileSync, cpSync } from 'node:fs'
import { createHash } from 'node:crypto'
import path from 'node:path'

// Include full license/notice texts, not just package license labels. No new dependency.
function licenseFiles(dir, recursive=false) {
 const found=[]
 for(const entry of readdirSync(dir,{withFileTypes:true})) {
  const file=path.join(dir,entry.name)
  if(entry.isDirectory() && recursive && !['.git','node_modules'].includes(entry.name)) found.push(...licenseFiles(file,true))
  if(entry.isFile() && /^(licen[sc]e|copying|notice|copyright|patents)([._-]|$)/i.test(entry.name)) found.push(file)
 }
 return found.sort()
}
export function noticeText(file) {
 const text=new TextDecoder('utf-8',{fatal:true}).decode(readFileSync(file))
 if(!text.trim() || text.includes('\uFFFD')) throw new Error(`Empty or damaged license text: ${file}`)
 return text
}

export function collectNpmNotices(frontend) {
 const lock=JSON.parse(readFileSync(path.join(frontend,'package-lock.json'),'utf8'))
 const packages=[]
 for(const [relative,info] of Object.entries(lock.packages)) {
  if(!relative||info.dev)continue
  const dir=path.join(frontend,relative)
  // Conservative: even an absent optional production package requires review.
  // The current Windows lockfile installs every production package.
  if(!existsSync(dir))throw new Error(`Missing production dependency ${relative}; run npm ci or review platform exclusions explicitly`)
  const installed=JSON.parse(readFileSync(path.join(dir,'package.json'),'utf8'))
  const expectedName=relative.split('node_modules/').at(-1)
  if(installed.name!==expectedName || installed.version!==info.version)throw new Error(`Installed dependency differs from lockfile: ${relative}`)
  if(expectedName==='victory-vendor' && info.version!=='37.3.6')throw new Error('Victory version changed; review the pinned supplemental license before packaging')
  const files=licenseFiles(dir,true)
  if(!files.length)throw new Error(`No license text found for ${relative}; review before packaging`)
  for(const file of files)noticeText(file)
  packages.push({label:`npm ${expectedName} ${info.version} (${info.license||'see text'})`,dir,files})
 }
 return packages
}
export function packageRelease({root,backend,frontend,outputDir,version}) {
 const sections=['THIRD-PARTY LICENSES AND NOTICES\nGenerated from the installed lockfile and Go module sources. Build-tool-only npm packages are excluded. Go module notices are conservatively included in full.']
 const append=(label,dir,files)=>{
  if(!files.length) throw new Error(`No license text found for ${label}; review before packaging`)
  sections.push(`\n===== ${label} =====\n`)
  for(const file of files)sections.push(`--- ${path.relative(dir,file).replaceAll('\\','/')} ---\n${noticeText(file)}`)
 }
 for(const p of collectNpmNotices(frontend))append(p.label,p.dir,p.files)
 const lines=execFileSync('go',['list','-m','-f','{{.Path}}\t{{.Version}}\t{{.Dir}}','all'],{cwd:backend,encoding:'utf8'}).trim().split(/\r?\n/)
 for(const line of lines){const [name,version,dir]=line.split('\t');if(!version)continue;if(!dir)throw new Error('Missing module source '+name);append(`Go module ${name} ${version}`,dir,licenseFiles(dir,true))}
 const goroot=execFileSync('go',['env','GOROOT'],{cwd:backend,encoding:'utf8'}).trim()
 append('Go runtime and standard library',goroot,licenseFiles(goroot))
 append('Go vendored standard-library dependencies',path.join(goroot,'src/vendor'),licenseFiles(path.join(goroot,'src/vendor'),true))
 append('Victory 37.3.6 upstream MIT license (npm distribution omits root license)',path.join(root,'docs/licenses'),[path.join(root,'docs/licenses/victory-37.3.6-LICENSE.txt')])
 writeFileSync(path.join(outputDir,'THIRD-PARTY-NOTICES.txt'),sections.join('\n\n'))
 for(const [source,dest] of [['LICENSE','LICENSE'],['docs/windows-usage.txt','README.txt'],['docs/release-notes.md','RELEASE-NOTES.md']])cpSync(path.join(root,source),path.join(outputDir,dest))
 const archive=path.join(root,'release',`storm-v${version}-windows-amd64.zip`)
 if(process.platform!=='win32')throw new Error('Windows archive packaging must be verified on Windows')
 const command="$files = @('storm.exe','LICENSE','README.txt','RELEASE-NOTES.md','THIRD-PARTY-NOTICES.txt','build-metadata.json') | ForEach-Object { Join-Path $env:STORM_PACKAGE_DIR $_ }; Compress-Archive -LiteralPath $files -DestinationPath $env:STORM_ARCHIVE -Force"
 execFileSync('powershell.exe',['-NoProfile','-NonInteractive','-Command',command],{env:{...process.env,STORM_PACKAGE_DIR:outputDir,STORM_ARCHIVE:archive},stdio:'inherit'})
 const digest=createHash('sha256').update(readFileSync(archive)).digest('hex')
 writeFileSync(archive+'.sha256',`${digest}  ${path.basename(archive)}\n`)
 console.log(`Packaged ${archive}\nSHA-256 ${digest}`)
}
