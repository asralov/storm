import { useId } from 'react'
import { formatMetric } from './formatMetric'

const colors = { PASS: '#34D399', FAIL: '#F87171', INCONCLUSIVE: '#FBBF24' }
const levelText = level => level ? `${level.target_rps} target req/s · stage ${level.stage_number}` : 'None observed'

export function AssessmentBadge({ status }) {
  if (!status) return null
  return <span className={`assessment-badge assessment-${status.toLowerCase()}`}><span aria-hidden="true">{status === 'PASS' ? '●' : status === 'FAIL' ? '◆' : '▲'}</span> {status}</span>
}

export function AssessmentDot({ cx, cy, payload, stroke }) {
  if (!Number.isFinite(cx) || !Number.isFinite(cy)) return null
  const status = payload?.assessment?.status
  const color = colors[status] || stroke
  return <g aria-label={status ? `Stage ${payload.number}: ${status}` : undefined}>
    {status && <title>{`Stage ${payload.number}: ${status}`}</title>}
    {status === 'FAIL' ? <path d={`M${cx},${cy - 5}l5,5 -5,5 -5,-5Z`} fill={color} /> : status === 'INCONCLUSIVE' ? <path d={`M${cx},${cy - 5}l5,9h-10Z`} fill={color} /> : <circle cx={cx} cy={cy} r={status ? 4 : 3} fill={color} />}
  </g>
}

export default function BreakpointAssessment({ analysis }) {
  const titleId = useId()
  if (!analysis) return null
  const result = analysis.breakpoint
  const thresholds = analysis.thresholds
  return <section className="panel breakpoint-panel" aria-labelledby={titleId}>
    <div className="details-heading"><h2 id={titleId}>Breakpoint Discovery</h2><span className="eyebrow">OBSERVED ASSESSMENT</span></div>
    <p className="breakpoint-message" role="status">{result.message}</p>
    <dl className="breakpoint-levels">
      <div><dt>Highest passing tested level</dt><dd>{levelText(analysis.highest_passing_stage)}</dd></div>
      <div><dt>First failing tested level</dt><dd>{levelText(analysis.first_failing_stage)}</dd></div>
      <div><dt>Bounded transition</dt><dd>{result.bounded && result.range ? `${result.range.lower_target_rps}–${result.range.upper_target_rps} target req/s` : 'Not established'}</dd></div>
    </dl>
    <p className="assessment-thresholds">P95 ≤ {formatMetric(thresholds.max_p95_latency_ms, 2)} ms · Error rate ≤ {formatMetric(thresholds.max_error_rate_percent, 2)}% · Achieved starts ≥ {formatMetric(thresholds.min_start_rate_percent, 2)}% of target</p>
    {result.reasons?.length > 0 && <ul className="assessment-reasons">{result.reasons.map(reason => <li key={reason}>{reason}</li>)}</ul>}
    <details className="assessment-method"><summary>How this assessment is calculated</summary>
      <p>Error rate: {analysis.sample_policy.error_rate_definition}.</p>
      <p>Offered load: {analysis.sample_policy.start_rate_definition}.</p>
      <p>Minimums: {analysis.sample_policy.min_error_outcomes} eligible outcomes for error rate; {analysis.sample_policy.min_latency_successes} successes for P95.</p>
    </details>
    {analysis.limitations.map(note => <p className="details-note" key={note}>{note}</p>)}
  </section>
}
