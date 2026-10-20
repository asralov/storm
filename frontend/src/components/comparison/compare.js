// Pure, read-only comparison of saved public report fields.
const finite = value => Number.isFinite(value) && value >= 0
const metric = value => finite(value) ? value : null
const mode = run => run?.config?.mode || 'standard'
const positive = value => Number.isFinite(value) && value > 0
const stagesOf = run => Array.isArray(run?.stages) ? run.stages : []
const method = value => typeof value === 'string' ? value.toUpperCase() : null
function validURL(value) { try { return typeof value === 'string' && new URL(value).protocol === 'http:' } catch { return false } }
function expectedTargets(config) {
  if (![config?.start_rps, config?.rps_increment, config?.max_rps].every(value => Number.isInteger(value) && value > 0) || config.max_rps < config.start_rps) return []
  const count = Math.floor((config.max_rps - config.start_rps) / config.rps_increment) + 1
  return count <= 10 ? Array.from({ length: count }, (_, i) => config.start_rps + i * config.rps_increment) : []
}
function loadDescription(run) {
  const c = run?.config
  if (mode(run) === 'standard') return positive(c?.rps) && positive(c?.duration_seconds) ? `${c.rps} req/s for ${c.duration_seconds} s` : null
  return expectedTargets(c).length && positive(c?.stage_duration_seconds) ? `${expectedTargets(c).join(', ')} req/s; ${c.stage_duration_seconds} s/stage` : null
}

export function delta(baseline, candidate) {
  if (!finite(baseline) || !finite(candidate)) return { difference: null, percent: null }
  const difference = candidate - baseline
  const percent = baseline === 0 ? null : difference / baseline * 100
  return { difference: Number.isFinite(difference) ? difference : null, percent: Number.isFinite(percent) ? percent : null }
}

export function toggleSelection(selected, item) {
  if (selected.some(run => run.id === item.id)) return selected.filter(run => run.id !== item.id)
  return selected.length < 2 ? [...selected, { id: item.id, url: item.url ?? item.config?.url, mode: item.mode ?? mode(item) }] : selected
}
export const swapSelection = selected => selected.length === 2 ? [selected[1], selected[0]] : selected
export function comparisonRoute(selected) {
  return selected.length === 2 && selected[0].id !== selected[1].id ? `#history/compare/${selected[0].id}/${selected[1].id}` : null
}
export function parseComparisonRoute(id = '') {
  const match = /^compare\/(run_[0-9a-f]{32})\/(run_[0-9a-f]{32})$/.exec(id)
  return match && match[1] !== match[2] ? [match[1], match[2]] : null
}

export function actualStarts(summary) {
  return finite(summary?.started_requests) && positive(summary?.load_window_seconds) ? metric(summary.started_requests / summary.load_window_seconds) : null
}
export function httpErrorRate(summary) {
  if (!finite(summary?.successes) || !finite(summary?.http_error_count)) return null
  const responses = summary.successes + summary.http_error_count
  return responses > 0 ? metric(summary.http_error_count / responses * 100) : null
}
const latency = (summary, key) => summary?.successes > 0 ? metric(summary[key]) : null
function validSummary(s) {
  return s && ['requests', 'started_requests', 'successes', 'failures', 'http_error_count', 'network_errors', 'canceled'].every(key => Number.isInteger(s[key]) && s[key] >= 0)
    && s.requests === s.successes + s.failures && s.failures === s.http_error_count + s.network_errors + s.canceled
    && s.started_requests === s.requests && positive(s.elapsed_seconds) && positive(s.load_window_seconds)
}

export function offeredLoadComparable(a, b, target) {
  const left = actualStarts(a), right = actualStarts(b)
  if (left === null || right === null || !positive(target) || a?.canceled > 0 || b?.canceled > 0) return false
  const x = left / target * 100, y = right / target * 100
  return x >= 95 && x <= 105 && y >= 95 && y <= 105 && Math.abs(x - y) <= 5
}

function thresholdText(config) {
  const t = config?.thresholds
  if (t == null) return 'Disabled'
  if (!positive(t.max_p95_latency_ms) || t.max_p95_latency_ms > 60000 || !finite(t.max_error_rate_percent) || t.max_error_rate_percent > 100 || !positive(t.min_start_rate_percent) || t.min_start_rate_percent > 100) return null
  return `P95 ${t.max_p95_latency_ms} ms; errors ${t.max_error_rate_percent}%; starts ${t.min_start_rate_percent}%`
}

export function compatibility(baseline, candidate) {
  const a = baseline?.config, b = candidate?.config
  const rows = []
  function field(label, left, right, valid = value => value !== undefined && value !== null && value !== '') {
    rows.push({ label, baseline: left ?? null, candidate: right ?? null, missing: !valid(left) || !valid(right), same: valid(left) && valid(right) && left === right })
  }
  field('Target URL (full path and query)', a?.url, b?.url, validURL)
  field('HTTP method', method(a?.method), method(b?.method))
  field('Test mode', mode(baseline), mode(candidate))
  field('Workers', a?.workers, b?.workers, Number.isInteger)
  if (mode(baseline) === 'standard' && mode(candidate) === 'standard') {
    field('Target RPS', a?.rps, b?.rps, positive)
    field('Configured duration (s)', a?.duration_seconds, b?.duration_seconds, positive)
  } else if (mode(baseline) === 'stepped' && mode(candidate) === 'stepped') {
    for (const [key, label] of [['start_rps', 'Starting RPS'], ['rps_increment', 'RPS increment'], ['max_rps', 'Maximum RPS'], ['stage_duration_seconds', 'Stage duration (s)']]) field(label, a?.[key], b?.[key], positive)
    field('Recorded stage targets', JSON.stringify(stagesOf(baseline).map(stage => stage.target_rps)), JSON.stringify(stagesOf(candidate).map(stage => stage.target_rps)))
  } else field('Configured load', loadDescription(baseline), loadDescription(candidate))
  field('Breakpoint thresholds', thresholdText(a), thresholdText(b))
  const reasons = []
  let insufficient = !a || !b || rows.some(row => row.missing) || !positive(a?.workers) || !positive(b?.workers)
  if (insufficient) reasons.push('Required configuration fields are missing or invalid.')
  if (baseline?.id && baseline.id === candidate?.id) { insufficient = true; reasons.push('Choose two distinct saved reports.') }
  if (!['standard', 'stepped'].includes(mode(baseline)) || !['standard', 'stepped'].includes(mode(candidate))) { insufficient = true; reasons.push('This test mode is not supported for comparison.') }
  for (const [label, run] of [['Baseline', baseline], ['Candidate', candidate]]) {
    if (run?.status !== 'completed') { insufficient = true; reasons.push(`${label} is ${run?.status || 'unavailable'}; partial results cannot support performance conclusions.`) }
    if (mode(run) === 'standard' && !validSummary(run?.summary)) { insufficient = true; reasons.push(`${label} has missing or inconsistent final metrics.`) }
    if (mode(run) === 'stepped') {
      const stages = stagesOf(run)
      const rates = stages.map(stage => stage.target_rps)
      const expected = expectedTargets(run.config)
      if (!stages.length || rates.some(rate => !positive(rate)) || new Set(rates).size !== rates.length || stages.some(stage => stage.status !== 'completed' || !validSummary(stage.summary))) {
        insufficient = true; reasons.push(`${label} has incomplete, invalid, or ambiguous stage results.`)
      }
      if (!expected.length || expected.length !== rates.length || expected.some((rate, i) => rate !== rates[i])) { insufficient = true; reasons.push(`${label}'s saved stage schedule does not match its recorded results.`) }
    }
  }
  const differences = rows.filter(row => !row.same)
  for (const row of differences) if (!row.missing) reasons.push(`${row.label} differs.`)
  const status = insufficient ? 'insufficient' : differences.length ? 'inspection_only' : 'comparable'
  let conclusions = status === 'comparable'
  if (mode(baseline) === 'standard' && mode(candidate) === 'standard' && conclusions && !offeredLoadComparable(baseline.summary, candidate.summary, a.rps)) {
    conclusions = false; reasons.push('Observed offered load differs materially or includes cancellations. Directional performance labels are suppressed.')
  }
  return { status, label: { comparable: 'Comparable configuration', inspection_only: 'Different configuration — inspection only', insufficient: 'Unsupported or insufficient data' }[status], conclusions, rows, reasons }
}

function metricRow(label, unit, left, right, preference, conclusions) {
  const baseline = metric(left), candidate = metric(right), change = delta(baseline, candidate)
  let interpretation = 'Neutral'
  if (baseline === null || candidate === null) interpretation = 'Unavailable'
  else if (change.difference === 0) interpretation = 'Unchanged'
  else if (conclusions && preference) {
    const favorable = preference === 'lower' ? candidate < baseline : candidate > baseline
    interpretation = `${candidate < baseline ? 'Lower' : 'Higher'} (${favorable ? 'favorable' : 'unfavorable'})`
  }
  return { label, unit, baseline, candidate, ...change, interpretation }
}

export function standardMetrics(a, b, conclusions = false) {
  const moreFailures = positive(a?.requests) && positive(b?.requests) && b.failures / b.requests > a.failures / a.requests
  return [
    ['Total requests', 'requests', a?.requests, b?.requests], ['Successful responses', 'requests', a?.successes, b?.successes],
    ['Failed requests', 'requests', a?.failures, b?.failures], ['Completed throughput', 'req/s', a?.throughput, b?.throughput, moreFailures ? null : 'higher'],
    ['Actual request-start rate', 'req/s', actualStarts(a), actualStarts(b)],
    ...[['avg_latency_ms', 'Average latency'], ['p50_latency_ms', 'P50 latency'], ['p95_latency_ms', 'P95 latency'], ['p99_latency_ms', 'P99 latency']].map(([key, label]) => [label, 'ms', latency(a, key), latency(b, key), 'lower']),
    ['HTTP error rate', '%', httpErrorRate(a), httpErrorRate(b), 'lower'], ['Missed scheduling opportunities', 'requests', a?.missed_schedules, b?.missed_schedules],
  ].map(([label, unit, left, right, preference]) => metricRow(label, unit, left, right, preference, conclusions))
}

// Use actual interval-end timestamps. Nulls break lines for missing windows and
// timestamps present only in the other run; no nearest-neighbor matching occurs.
export function alignThroughput(a = [], b = []) {
  const points = new Map()
  function add(intervals, key) {
    let previousEnd = null
    for (const sample of intervals ?? []) {
      if (!finite(sample.elapsed_seconds) || !positive(sample.duration_seconds)) continue
      const end = sample.elapsed_seconds
      if (previousEnd !== null && finite(sample.start_seconds) && sample.start_seconds > previousEnd + 1e-6) {
        if (!points.has(sample.start_seconds)) points.set(sample.start_seconds, { elapsed: sample.start_seconds, baseline: null, candidate: null })
      }
      if (!points.has(end)) points.set(end, { elapsed: end, baseline: null, candidate: null })
      points.get(end)[key] = metric(sample.throughput)
      previousEnd = end
    }
  }
  add(a, 'baseline'); add(b, 'candidate')
  return [...points.values()].sort((left, right) => left.elapsed - right.elapsed)
}

export function matchingStages(a = [], b = [], conclusions = false) {
  const unique = stages => {
    const counts = new Map()
    for (const stage of stages ?? []) if (positive(stage.target_rps)) counts.set(stage.target_rps, (counts.get(stage.target_rps) || 0) + 1)
    return new Map((stages ?? []).filter(stage => counts.get(stage.target_rps) === 1).map(stage => [stage.target_rps, stage]))
  }
  const left = unique(a), right = unique(b)
  const common = [...left.keys()].filter(rate => right.has(rate)).sort((x, y) => x - y).map(target => {
    const baseline = left.get(target), candidate = right.get(target)
    const eligible = conclusions && baseline.status === 'completed' && candidate.status === 'completed' && offeredLoadComparable(baseline.summary, candidate.summary, target)
    return { target, baselineStatus: baseline.status, candidateStatus: candidate.status, conclusions: eligible,
      p95: metricRow('P95 latency', 'ms', latency(baseline.summary, 'p95_latency_ms'), latency(candidate.summary, 'p95_latency_ms'), 'lower', eligible),
      starts: metricRow('Actual request-start rate', 'req/s', actualStarts(baseline.summary), actualStarts(candidate.summary), null, eligible),
    }
  })
  return { common, baselineOnly: [...left.keys()].filter(rate => !right.has(rate)).sort((x, y) => x - y), candidateOnly: [...right.keys()].filter(rate => !left.has(rate)).sort((x, y) => x - y) }
}

export function compareRuns(baseline, candidate) {
  const config = compatibility(baseline, candidate)
  const standard = mode(baseline) === 'standard' && mode(candidate) === 'standard'
  const stepped = mode(baseline) === 'stepped' && mode(candidate) === 'stepped'
  return {
    compatibility: config, standard, stepped,
    metrics: standard ? standardMetrics(baseline?.summary, candidate?.summary, config.conclusions) : [],
    latencyBars: standard ? ['p50', 'p95', 'p99'].map(key => ({ label: key.toUpperCase(), baseline: latency(baseline?.summary, `${key}_latency_ms`), candidate: latency(candidate?.summary, `${key}_latency_ms`) })) : [],
    throughput: standard && config.status === 'comparable' ? alignThroughput(baseline?.intervals, candidate?.intervals) : [],
    stages: stepped ? matchingStages(stagesOf(baseline), stagesOf(candidate), config.conclusions) : { common: [], baselineOnly: [], candidateOnly: [] },
    breakpoints: { baseline: baseline?.analysis?.breakpoint?.status ? baseline.analysis : null, candidate: candidate?.analysis?.breakpoint?.status ? candidate.analysis : null },
  }
}
