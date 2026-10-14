import test from 'node:test'
import assert from 'node:assert/strict'
import { mergeIntervals, mergeRun, MAX_SAMPLES } from './timeline.js'

test('stage transitions reset interval sequences within the same run', () => {
  const previous = mergeRun(null, { id: 'a', current_stage: 1, intervals: [interval(1), interval(2)] })
  const next = mergeRun(previous, { id: 'a', current_stage: 2, intervals: [interval(1)] })
  assert.equal(next.timeline.length, 1)
  assert.equal(next.current_stage, 2)
})

const interval = sequence => ({ sequence, start_seconds: sequence - 1, elapsed_seconds: sequence, duration_seconds: 1, completed: 10, successes: 8, failures: 2, http_errors: 1, network_errors: 1, canceled: 0, throughput: 10, avg_latency_ms: 20, p50_latency_ms: 10, p95_latency_ms: 40, p99_latency_ms: 50 })

test('history survives missed polls and can be recovered on a fresh page', () => {
  const data = { id: 'a', status: 'running', intervals: [interval(1), interval(2), interval(3)] }
  assert.equal(mergeRun(null, data).timeline.length, 3)
  const previous = mergeRun(null, { ...data, intervals: [interval(1)] })
  assert.deepEqual(mergeRun(previous, data).timeline, mergeRun(null, data).timeline)
})
test('same sequence deduplicates and late outcomes correct a window', () => {
  const points = mergeIntervals([], [interval(1)])
  assert.equal(mergeIntervals(points, [interval(1)]).length, 1)
  assert.equal(mergeIntervals(points, [{ ...interval(1), successes: 9 }])[0].successes, 9)
})
test('final partial window preserves measured throughput and interval statistics', () => {
  const final = mergeRun(null, { id: 'a', status: 'completed', intervals: [{ ...interval(1), elapsed_seconds: 0.5, duration_seconds: 0.5, throughput: 20 }] })
  assert.equal(final.timeline[0].throughput, 20)
  assert.equal(final.timeline[0].p95, 40)
  assert.equal(final.timeline[0].failures, 2)
})
test('missing latency and breakdowns stay unavailable', () => {
  const point = mergeIntervals([], [{ ...interval(1), successes: 0, http_errors: undefined }])[0]
  assert.equal(point.p50, null)
  assert.equal(point.average, null)
  assert.equal(point.httpErrors, null)
})
test('buffer is bounded and new runs reset history', () => {
  const points = mergeIntervals([], Array.from({ length: MAX_SAMPLES + 10 }, (_, i) => interval(i + 1)))
  assert.equal(points.length, MAX_SAMPLES)
  assert.equal(points[0].sequence, 11)
  assert.deepEqual(mergeRun({ id: 'a', timeline: points }, { id: 'b', intervals: [] }).timeline, [])
})
