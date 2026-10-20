import test, { before, after } from 'node:test'
import assert from 'node:assert/strict'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { createServer } from 'vite'

let server, ComparisonView, HistoryListView
before(async () => {
  server = await createServer({ cacheDir: 'node_modules/.vite-comparison-tests', optimizeDeps: { noDiscovery: true, include: [] }, server: { middlewareMode: true, hmr: false, ws: false, watch: null }, appType: 'custom' })
  ComparisonView = (await server.ssrLoadModule('/src/components/comparison/CompareRuns.jsx')).ComparisonView
  HistoryListView = (await server.ssrLoadModule('/src/components/history/HistoryPage.jsx')).HistoryListView
})
after(async () => { await server?.close() })

const makeRun = id => ({ id, status: 'completed', config: { mode: 'standard', url: 'http://127.0.0.1:8081/fast', method: 'GET', workers: 4, rps: 20, duration_seconds: 10 }, summary: { requests: 200, successes: 200, failures: 0, started_requests: 200, http_error_count: 0, network_errors: 0, canceled: 0, elapsed_seconds: 10, load_window_seconds: 10, throughput: 20, p95_latency_ms: 100 } })

test('selection requires two reports and identifies their roles', () => {
  const zero = renderToStaticMarkup(createElement(HistoryListView, { loading: false, page: { items: [], total: 0 } }))
  assert.match(zero, /<button disabled="">Compare Runs<\/button>/)
  const two = renderToStaticMarkup(createElement(HistoryListView, { loading: false, page: { items: [], total: 0 }, selection: [{ id: 'a', url: 'Baseline target' }, { id: 'b', url: 'Candidate target' }] }))
  assert.match(two, /<button>Compare Runs<\/button>/)
  assert.ok(two.includes('Baseline target') && two.includes('Candidate target') && two.includes('Swap selection'))
})

test('compatible comparisons expose values, caveats, and navigation', () => {
  const html = renderToStaticMarkup(createElement(ComparisonView, { baseline: makeRun('a'), candidate: makeRun('b') }))
  assert.ok(html.includes('Comparable configuration'))
  assert.ok(html.includes('Swap Baseline and Candidate'))
  assert.ok(html.includes('Candidate − Baseline'))
  assert.ok(html.includes('Matching configuration does not prove identical'))
  assert.ok(html.includes('Both runs need saved breakpoint assessments'))
})

test('mismatched and interrupted runs show explicit warnings without favorable verdicts', () => {
  const a = makeRun('a'), b = makeRun('b'); b.config.url += '?different=1'; b.summary.p95_latency_ms = 1
  const mismatch = renderToStaticMarkup(createElement(ComparisonView, { baseline: a, candidate: b }))
  assert.ok(mismatch.includes('inspection only'))
  assert.ok(mismatch.includes('Target URL (full path and query) differs.'))
  assert.ok(!mismatch.includes('(favorable)'))
  b.status = 'interrupted'; delete b.summary
  const interrupted = renderToStaticMarkup(createElement(ComparisonView, { baseline: a, candidate: b }))
  assert.ok(interrupted.includes('Unsupported or insufficient data'))
  assert.ok(interrupted.includes('Candidate is interrupted'))
  assert.ok(interrupted.includes('N/A'))
})

test('breakpoint comparison uses saved observations without capacity claims', () => {
  const a = makeRun('a'), b = makeRun('b')
  a.analysis = { highest_passing_stage: { target_rps: 20, stage_number: 1 }, first_failing_stage: null, breakpoint: { status: 'not_reached', message: 'No crossing observed.' } }
  b.analysis = { highest_passing_stage: null, first_failing_stage: { target_rps: 20, stage_number: 1 }, breakpoint: { status: 'partial', message: 'Partial observations only.' } }
  const html = renderToStaticMarkup(createElement(ComparisonView, { baseline: a, candidate: b }))
  assert.ok(html.includes('No crossing observed.'))
  assert.ok(html.includes('Partial observations only.'))
  assert.ok(html.includes('None observed'))
  assert.ok(html.includes('does not establish an increase in sustainable capacity'))
})
