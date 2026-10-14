import test from 'node:test'
import assert from 'node:assert/strict'
import { stagePlan } from './stagePlan.js'

test('maximum is a ceiling and total uses generated stage count', () => {
  assert.deepEqual(stagePlan(10, 10, 35, 5), { rates: [10, 20, 30], totalSeconds: 15, error: '' })
  assert.deepEqual(stagePlan(10, 20, 10, 1).rates, [10])
})
test('invalid and excessive plans are rejected', () => {
  for (const args of [[0, 10, 30, 5], [10, 0, 30, 5], [30, 10, 10, 5], [10, 10, 101, 5], [1, 1, 20, 1], [10, 10, 50, 30], [10, 10, 30, 31], [10.5, 10, 30, 5]]) {
    assert.ok(stagePlan(...args).error, String(args))
  }
})
