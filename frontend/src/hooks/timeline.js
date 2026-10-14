export const MAX_SAMPLES = 300
const numberOrNull = value => Number.isFinite(value) ? value : null

// The API supplies measured windows, including the final partial window.
// Upsert by sequence: a late-delivered outcome can correct an earlier window.
export function mergeIntervals(previous, intervals = []) {
  const points = new Map(previous.map(point => [point.sequence, point]))
  for (const interval of intervals ?? []) {
    if (!Number.isInteger(interval.sequence) || interval.sequence < 1 || !Number.isFinite(interval.elapsed_seconds) || !(interval.duration_seconds > 0)) continue
    const latency = key => interval.successes > 0 ? numberOrNull(interval[key]) : null
    points.set(interval.sequence, {
      sequence: interval.sequence,
      start: interval.start_seconds,
      elapsed: interval.elapsed_seconds,
      duration: interval.duration_seconds,
      completed: interval.completed,
      successes: interval.successes,
      throughput: numberOrNull(interval.throughput),
      average: latency('avg_latency_ms'),
      p50: latency('p50_latency_ms'),
      p95: latency('p95_latency_ms'),
      p99: latency('p99_latency_ms'),
      failures: numberOrNull(interval.failures),
      httpErrors: numberOrNull(interval.http_errors),
      networkErrors: numberOrNull(interval.network_errors),
      canceled: numberOrNull(interval.canceled),
    })
  }
  return [...points.values()].sort((a, b) => a.sequence - b.sequence).slice(-MAX_SAMPLES)
}

export function mergeRun(previous, incoming) {
  if (!incoming) return null
  const same = previous?.id === incoming.id
  const sameStage = same && previous.current_stage === incoming.current_stage
  return {
    ...incoming,
    chartRecovered: incoming.chartRecovered ?? (same && previous.chartRecovered),
    timeline: mergeIntervals(sameStage ? previous.timeline ?? [] : [], incoming.intervals),
  }
}
