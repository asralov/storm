import test, { before, after } from 'node:test'
import assert from 'node:assert/strict'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { createServer } from 'vite'

let server, HistoryListView, SavedRunDetail, PersistenceNotice, api, deleteSavedReport
before(async () => {
  server = await createServer({ cacheDir: 'node_modules/.vite-history-tests', optimizeDeps: { noDiscovery: true, include: [] }, server: { middlewareMode: true, hmr: false, ws: false, watch: null }, appType: 'custom' })
  const views = await server.ssrLoadModule('/src/components/history/HistoryPage.jsx')
  HistoryListView = views.HistoryListView
  SavedRunDetail = views.SavedRunDetail
  PersistenceNotice = (await server.ssrLoadModule('/src/components/dashboard/RunReport.jsx')).PersistenceNotice
  api = await server.ssrLoadModule('/src/services/testService.js')
  deleteSavedReport = (await server.ssrLoadModule('/src/services/historyActions.js')).deleteSavedReport
})
after(async () => { await server?.close() })

const id = `run_${'a'.repeat(32)}`
const run = { id, status: 'completed', config: { mode: 'standard', url: 'http://127.0.0.1:8081/fast', method: 'GET', workers: 2, rps: 20 }, started_at: '2026-01-01T00:00:00Z', completed_at: '2026-01-01T00:00:01Z', intervals: [], summary: { requests: 19, successes: 19, failures: 0, throughput: 19, avg_latency_ms: 1, p50_latency_ms: 1, p95_latency_ms: 2, p99_latency_ms: 3 } }

test('history loading, empty state, and API failure states render clearly', () => {
  const loading = renderToStaticMarkup(createElement(HistoryListView, { loading: true }))
  assert.ok(loading.includes('Loading saved tests'))
  const empty = renderToStaticMarkup(createElement(HistoryListView, { loading: false, page: { items: [], total: 0 } }))
  assert.ok(empty.includes('Your test history starts here'))
  const failed = renderToStaticMarkup(createElement(HistoryListView, { error: 'Storage unavailable', loading: false }))
  assert.ok(failed.includes('role="alert"'))
  assert.ok(failed.includes('Storage unavailable'))
  assert.ok(failed.includes('Try again'))
})

test('history rows show metadata and read-only report links', () => {
  const item = { id, url: run.config.url, mode: 'stepped', configured_load: '10–30 req/s', status: 'completed', total_requests: 50, p95_label: 'Max stage P95', p95_latency_ms: 100, breakpoint_status: 'not_reached' }
  const html = renderToStaticMarkup(createElement(HistoryListView, { page: { items: [item], total: 1 } }))
  assert.ok(html.includes(`#history/${id}`))
  assert.ok(html.includes('Max stage P95'))
  assert.ok(html.includes('not reached'))
  assert.ok(html.includes(`Delete saved run ${id}`))
})

test('saved detail preserves metrics without execution controls', () => {
  const html = renderToStaticMarkup(createElement(SavedRunDetail, { run }))
  assert.ok(html.includes('Read-only'))
  assert.ok(html.includes(run.config.url))
  assert.ok(html.includes('Total requests'))
  assert.ok(!html.includes('Run test'))
  assert.ok(!html.includes('Stop test'))
  const interrupted = renderToStaticMarkup(createElement(SavedRunDetail, { run: { ...run, status: 'interrupted', summary: undefined, completed_at: null, error: 'No final report was recovered.' } }))
  assert.ok(interrupted.includes('No final report was recovered.'))
  assert.ok(!interrupted.includes('Total requests'))
})

test('deletion requires confirmation and surfaces storage errors', async () => {
  const calls = []
  assert.equal(await deleteSavedReport(run, () => false, async value => calls.push(value)), false)
  assert.equal(calls.length, 0)
  assert.equal(await deleteSavedReport(run, question => question.includes('cannot be undone'), async value => calls.push(value)), true)
  assert.deepEqual(calls, [id])
  await assert.rejects(deleteSavedReport(run, () => true, async () => { throw new Error('database unavailable') }), /database unavailable/)
})

test('history API requests are paginated and deletion is a JSON mutation', async t => {
  const calls = []
  t.mock.method(globalThis, 'fetch', async (url, options) => { calls.push({ url, options }); return new Response(JSON.stringify({ items: [], total: 0 }), { status: 200 }) })
  await api.listHistory(20)
  await api.getHistory(id)
  await api.deleteHistory(id)
  assert.equal(calls[0].url, '/api/history?limit=20&offset=20')
  assert.equal(calls[1].url, `/api/history/${id}`)
  assert.equal(calls[2].options.method, 'DELETE')
  assert.equal(calls[2].options.headers['Content-Type'], 'application/json')
})

test('API missing records and persistence failures are not hidden', async t => {
  t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ error: 'saved run not found' }), { status: 404 }))
  await assert.rejects(api.getHistory(id), err => err.status === 404 && err.message === 'saved run not found')
  const html = renderToStaticMarkup(createElement(PersistenceNotice, { run: { ...run, persistence: { status: 'failed', error: 'The report could not be saved.' } } }))
  assert.ok(html.includes('role="alert"'))
  assert.ok(html.includes('could not be saved'))
  const deleted = renderToStaticMarkup(createElement(PersistenceNotice, { run: { ...run, persistence: { status: 'deleted' } } }))
  assert.ok(deleted.includes('deleted from history'))
  assert.ok(!deleted.includes('View saved report'))
})

test('untrusted target text is escaped in saved reports', () => {
  const hostile = '<img src=x onerror=alert(1)>'
  const html = renderToStaticMarkup(createElement(SavedRunDetail, { run: { ...run, config: { ...run.config, url: hostile } } }))
  assert.ok(!html.includes('<img src=x'))
  assert.ok(html.includes('&lt;img'))
})
