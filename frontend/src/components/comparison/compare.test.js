import test from 'node:test'
import assert from 'node:assert/strict'
import { actualStarts, alignThroughput, compareRuns, compatibility, comparisonRoute, delta, httpErrorRate, matchingStages, offeredLoadComparable, parseComparisonRoute, swapSelection, toggleSelection } from './compare.js'

const id = n => `run_${String(n).padStart(32, '0')}`
function summary(overrides = {}) {
  return { requests: 200, started_requests: 200, successes: 200, failures: 0, http_error_count: 0, network_errors: 0, canceled: 0, elapsed_seconds: 11, load_window_seconds: 10, throughput: 200 / 11, avg_latency_ms: 20, p50_latency_ms: 10, p95_latency_ms: 40, p99_latency_ms: 60, missed_schedules: 0, ...overrides }
}
function standard(n = 1) { return { id: id(n), status: 'completed', config: { mode: 'standard', url: 'http://127.0.0.1:8081/fast?a=1', method: 'GET', workers: 4, rps: 20, duration_seconds: 10 }, summary: summary(), intervals: [{ sequence: 1, start_seconds: 0, elapsed_seconds: 1, duration_seconds: 1, throughput: 19 }] } }
function stepped(n, rates) { return { id: id(n), status: 'completed', config: { mode: 'stepped', url: 'http://127.0.0.1:8081/fast', method: 'GET', workers: 4, start_rps: rates[0], max_rps: rates.at(-1), rps_increment: 10, stage_duration_seconds: 10 }, stages: rates.map((target_rps, index) => ({ number: index + 1, status: 'completed', target_rps, summary: summary({ requests: target_rps * 10, started_requests: target_rps * 10, successes: target_rps * 10 }) })) } }
function freeze(value) { if (value && typeof value === 'object') { Object.freeze(value); Object.values(value).forEach(freeze) } return value }

test('selection is bounded, can be swapped, and produces a recoverable route', () => {
  const initial = freeze([])
  const one = toggleSelection(initial, standard(1)), two = toggleSelection(one, standard(2))
  assert.equal(comparisonRoute(one), null)
  assert.equal(toggleSelection(two, standard(3)), two)
  const swapped = swapSelection(two)
  assert.deepEqual(swapped.map(run => run.id), [id(2), id(1)])
  assert.deepEqual(parseComparisonRoute(comparisonRoute(swapped).slice(9)), [id(2), id(1)])
  assert.equal(toggleSelection(two, standard(1)).length, 1)
  assert.equal(parseComparisonRoute(`compare/${id(1)}/${id(1)}`), null)
  assert.equal(parseComparisonRoute('compare/bad/missing'), null)
})

test('differences are signed and percentages never divide by zero or missing values', () => {
  assert.deepEqual(delta(100, 80), { difference: -20, percent: -20 })
  assert.deepEqual(delta(0, 10), { difference: 10, percent: null })
  assert.deepEqual(delta(0, 0), { difference: 0, percent: null })
  assert.deepEqual(delta(null, 10), { difference: null, percent: null })
  assert.deepEqual(delta(1, Infinity), { difference: null, percent: null })
})

test('compatible standard runs preserve units and neutral counts', () => {
  const a = standard(1), b = standard(2)
  b.summary = summary({ p95_latency_ms: 20, throughput: 19 })
  const result = compareRuns(freeze(a), freeze(b))
  assert.equal(result.compatibility.status, 'comparable')
  assert.equal(result.metrics.length, 11)
  assert.equal(result.metrics.find(row => row.label === 'P95 latency').interpretation, 'Lower (favorable)')
  assert.equal(result.metrics.find(row => row.label === 'Completed throughput').interpretation, 'Higher (favorable)')
  assert.equal(result.metrics.find(row => row.label === 'Total requests').interpretation, 'Unchanged')
})

test('each material configuration difference suppresses conclusions', () => {
  for (const patch of [{ url: 'http://127.0.0.1:8081/slow?a=1' }, { url: 'http://127.0.0.1:8081/fast?a=2' }, { workers: 5 }, { rps: 30 }, { duration_seconds: 20 }, { method: 'POST' }, { thresholds: { max_p95_latency_ms: 200, max_error_rate_percent: 1, min_start_rate_percent: 95 } }]) {
    const a = standard(1), b = standard(2); Object.assign(b.config, patch); b.summary.p95_latency_ms = 10
    const result = compareRuns(a, b)
    assert.equal(result.compatibility.status, 'inspection_only', JSON.stringify(patch))
    assert.equal(result.compatibility.conclusions, false)
    assert.equal(result.metrics.find(row => row.label === 'P95 latency').interpretation, 'Neutral')
    assert.deepEqual(result.throughput, [])
  }
})

test('canceled, interrupted, and missing reports cannot imply performance improvement', () => {
  for (const status of ['canceled', 'interrupted', 'failed']) { const b = standard(2); b.status = status; assert.equal(compatibility(standard(1), b).status, 'insufficient') }
  const b = standard(2); delete b.summary
  assert.equal(compareRuns(standard(1), b).metrics[0].candidate, null)
  assert.equal(compatibility(null, b).status, 'insufficient')
  assert.equal(compareRuns(standard(1), { ...standard(2), analysis: {} }).breakpoints.candidate, null)
})

test('request-start and HTTP error-rate denominators are explicit', () => {
  assert.equal(actualStarts(summary()), 20) // Total execution is 11s, load is 10s.
  assert.equal(httpErrorRate(summary({ successes: 90, http_error_count: 10, network_errors: 20, canceled: 5 })), 10)
  assert.equal(httpErrorRate(summary({ successes: 0, http_error_count: 0 })), null)
  assert.equal(offeredLoadComparable(summary(), summary({ started_requests: 100 }), 20), false)
  const b = standard(2); b.summary = summary({ requests: 100, started_requests: 100, successes: 100 })
  assert.equal(compareRuns(standard(1), b).compatibility.conclusions, false)
  const zero = standard(2); zero.summary = summary({ successes: 0, failures: 200, http_error_count: 200, avg_latency_ms: 0, p95_latency_ms: 0 })
  assert.equal(compareRuns(standard(1), zero).metrics.find(row => row.label === 'P95 latency').candidate, null)
})

test('throughput uses exact elapsed times and keeps missing samples as null', () => {
  const a = freeze([{ start_seconds: 0, elapsed_seconds: 1, duration_seconds: 1, throughput: 20 }, { start_seconds: 2, elapsed_seconds: 3, duration_seconds: 1, throughput: 10 }])
  const b = freeze([{ start_seconds: 0, elapsed_seconds: 1.2, duration_seconds: 1.2, throughput: 25 }])
  assert.deepEqual(alignThroughput(a, b), [{ elapsed: 1, baseline: 20, candidate: null }, { elapsed: 1.2, baseline: null, candidate: 25 }, { elapsed: 2, baseline: null, candidate: null }, { elapsed: 3, baseline: 10, candidate: null }])
})

test('stepped comparisons pair only common RPS levels and flag workload differences', () => {
  const a = stepped(1, [10, 20]), b = stepped(2, [20, 30])
  const result = compareRuns(freeze(a), freeze(b))
  assert.equal(result.compatibility.status, 'inspection_only')
  assert.deepEqual(result.stages.common.map(stage => stage.target), [20])
  assert.deepEqual(result.stages.baselineOnly, [10]); assert.deepEqual(result.stages.candidateOnly, [30])
  assert.equal(result.stages.common[0].conclusions, false)
  const identical = compareRuns(stepped(1, [10, 20]), stepped(2, [10, 20]))
  assert.equal(identical.compatibility.status, 'comparable')
  assert.equal(identical.stages.common[0].conclusions, true)
  assert.equal(matchingStages([{ target_rps: 10 }, { target_rps: 10 }], [{ target_rps: 10 }]).common.length, 0)
})

test('comparisons do not mutate reports or synthesize missing breakpoint observations', () => {
  const a = freeze(standard(1)), b = freeze(standard(2)), before = JSON.stringify([a, b])
  const result = compareRuns(a, b)
  assert.deepEqual(result.breakpoints, { baseline: null, candidate: null })
  assert.equal(JSON.stringify([a, b]), before)
})

test('missing scheduled stages and malformed configuration are insufficient', () => {
  const a = stepped(1, [10, 20]), b = stepped(2, [10, 20]); b.stages.pop()
  assert.equal(compatibility(a, b).status, 'insufficient')
  const bad = standard(2); bad.config.url = 'not a URL'
  assert.equal(compatibility(standard(1), bad).status, 'insufficient')
  const mixed = compareRuns(standard(1), stepped(2, [10, 20]))
  assert.equal(mixed.compatibility.status, 'inspection_only')
  assert.ok(mixed.compatibility.rows.some(row => row.label === 'Configured load'))
})
