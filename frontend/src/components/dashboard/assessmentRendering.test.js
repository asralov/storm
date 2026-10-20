import test, { before, after } from 'node:test'
import assert from 'node:assert/strict'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { createServer } from 'vite'

let server, SteppedResults, TestConfig, AssessmentDot
before(async () => {
  server = await createServer({ cacheDir: 'node_modules/.vite-assessment-tests', optimizeDeps: { noDiscovery: true, include: [] }, server: { middlewareMode: true, hmr: false, ws: false, watch: null }, appType: 'custom' })
  SteppedResults = (await server.ssrLoadModule('/src/components/dashboard/SteppedResults.jsx')).default
  TestConfig = (await server.ssrLoadModule('/src/components/dashboard/TestConfig.jsx')).default
  AssessmentDot = (await server.ssrLoadModule('/src/components/dashboard/BreakpointAssessment.jsx')).AssessmentDot
})
after(async () => { await server?.close() })

const thresholds = { max_p95_latency_ms: 200, max_error_rate_percent: 1, min_start_rate_percent: 95 }
const statuses = ['PASS', 'FAIL', 'INCONCLUSIVE']
const stages = statuses.map((_, index) => ({ number: index + 1, target_rps: 20 + index * 10, status: 'completed', actual_start_rps: 20, summary: { successes: 200, failures: 0, throughput: 19, p50_latency_ms: 50, p95_latency_ms: 100, p99_latency_ms: 120, missed_schedules: 0 } }))
const analysis = {
  thresholds,
  sample_policy: { min_error_outcomes: 100, min_latency_successes: 100, error_rate_definition: 'Eligible errors / eligible outcomes', start_rate_definition: 'Starts / load seconds / target' },
  stages: statuses.map((status, index) => ({ stage_number: index + 1, target_rps: 20 + index * 10, status, reasons: [`Backend explanation for ${status}`], metrics_available: true, eligible_outcomes: 200, successful_samples: 200, achieved_start_percent: 100, error_rate_percent: 0 })),
  highest_passing_stage: { stage_number: 1, target_rps: 20 },
  first_failing_stage: { stage_number: 2, target_rps: 30 },
  breakpoint: { status: 'inconclusive', message: 'No bounded conclusion is supported.', bounded: false, reasons: ['A stage is inconclusive.'] },
  limitations: ['Observed criteria are not a statistically validated SLO.'],
}

test('all three assessment states and backend explanations render in the table', () => {
  const html = renderToStaticMarkup(createElement(SteppedResults, { run: { stages, analysis } }))
  for (const status of statuses) {
    assert.ok(html.includes(`data-assessment="${status}"`))
    assert.ok(html.includes(`Backend explanation for ${status}`))
  }
  assert.ok(html.includes('No bounded conclusion is supported.'))
  assert.ok(html.includes('Not established'))
  assert.ok(html.includes('20 target req/s'))
  assert.ok(html.includes('Observed criteria are not a statistically validated SLO.'))
})

test('bounded range and partial messages display only what the backend supplies', () => {
  const report = { ...analysis, breakpoint: { status: 'bounded', message: 'Observed threshold crossing between 20 and 30 target RPS.', bounded: true, range: { lower_target_rps: 20, upper_target_rps: 30 }, reasons: [] } }
  const html = renderToStaticMarkup(createElement(SteppedResults, { run: { stages, analysis: report } }))
  assert.ok(html.includes('20–30 target req/s'))
  const partial = renderToStaticMarkup(createElement(SteppedResults, { run: { stages, analysis: { ...report, breakpoint: { status: 'partial', message: 'Partial results only.', bounded: false, reasons: [] } } } }))
  assert.ok(partial.includes('Partial results only.'))
  assert.ok(!partial.includes('20–30 target req/s'))
})

test('legacy stepped results and standard form do not enable analysis', () => {
  const html = renderToStaticMarkup(createElement(SteppedResults, { run: { stages } }))
  assert.ok(!html.includes('Breakpoint Discovery'))
  assert.ok(!html.includes('data-assessment='))
  const form = renderToStaticMarkup(createElement(TestConfig, { initialConfig: { mode: 'standard' }, onSubmit: () => {} }))
  assert.ok(!form.includes('Enable Breakpoint Discovery'))
})

test('restored threshold settings render their saved values', () => {
  const form = renderToStaticMarkup(createElement(TestConfig, { initialConfig: { mode: 'stepped', thresholds: { ...thresholds, max_p95_latency_ms: 345 } }, onSubmit: () => {} }))
  assert.ok(form.includes('Enable Breakpoint Discovery'))
  assert.ok(form.includes('value="345"'))
  assert.ok(form.includes('100 eligible outcomes'))
})

test('assessment markers use shapes and accessible labels as well as colors', () => {
  for (const status of statuses) {
    const svg = renderToStaticMarkup(createElement('svg', null, createElement(AssessmentDot, { cx: 10, cy: 20, stroke: 'blue', payload: { number: 1, assessment: { status } } })))
    assert.ok(svg.includes(`aria-label="Stage 1: ${status}"`))
    assert.ok(svg.includes(status === 'PASS' ? '<circle' : '<path'))
  }
})
