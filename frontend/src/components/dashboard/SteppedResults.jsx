import { useId } from 'react'
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis, Legend } from 'recharts'
import { formatMetric } from './formatMetric'
import BreakpointAssessment, { AssessmentBadge, AssessmentDot } from './BreakpointAssessment'

const time = value => value ? new Date(value).toLocaleTimeString() : '—'

function ComparisonTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null
  const assessment = payload[0].payload.assessment
  return <div className="chart-tooltip"><p>Target starts: {label} req/s</p>{payload.map(series => <p key={series.dataKey}>{series.name}: {formatMetric(series.value, 2)}</p>)}{assessment && <AssessmentBadge status={assessment.status} />}</div>
}

export default function SteppedResults({ run }) {
  const titleId = useId()
  const stages = run.stages ?? []
  const assessments = new Map((run.analysis?.stages ?? []).map(item => [item.stage_number, item]))
  // Partial canceled stages remain in the table, but do not masquerade as full
  // duration measurements in the comparison charts.
  const completed = stages.filter(stage => stage.status === 'completed' && stage.summary)
  const data = completed.map(stage => ({
    number: stage.number, assessment: assessments.get(stage.number),
    target: stage.target_rps, started: stage.actual_start_rps, throughput: stage.summary.throughput,
    p95: stage.summary.successes > 0 ? stage.summary.p95_latency_ms : null,
  }))
  return <section className="stage-results" aria-labelledby={titleId}>
    <BreakpointAssessment analysis={run.analysis} />
    <div className="details-heading"><h2 id={titleId}>Stage comparisons</h2><span className="eyebrow">{completed.length} / {stages.length} COMPLETED</span></div>
    <p className="timeline-caption">Each point is a completed stage. Requests and latency samples are independent across stages. Partial and skipped stages are shown in the table.</p>
    {run.analysis && <div className="assessment-legend" aria-label="Stage assessment markers">{['PASS', 'FAIL', 'INCONCLUSIVE'].map(status => <AssessmentBadge key={status} status={status} />)}</div>}
    <div className="charts-grid">{[
      { title: 'P95 latency vs target RPS', series: [['p95', 'P95 latency (ms)', '#FBBF24']] },
      { title: 'Target and achieved rates', series: [['target', 'Target starts (req/s)', '#94A3B8'], ['started', 'Actual starts (req/s)', '#60A5FA'], ['throughput', 'Completed (req/s)', '#34D399']] },
    ].map(chart => <article className="panel chart-panel" key={chart.title} aria-label={chart.title}>
      <h3>{chart.title}</h3>
      <div className="chart-canvas">{data.some(point => chart.series.some(([key]) => Number.isFinite(point[key]))) ? <ResponsiveContainer width="100%" height="100%" minWidth={0}>
        <LineChart data={data} accessibilityLayer title={chart.title} margin={{ top: 16, right: 18, bottom: 5, left: 0 }}>
          <CartesianGrid stroke="#253047" strokeDasharray="3 5" vertical={false} />
          <XAxis type="number" dataKey="target" domain={['dataMin', 'dataMax']} tickFormatter={v => `${v} req/s`} tick={{ fontSize: 10 }} stroke="#94A3B8" />
          <YAxis domain={[0, 'auto']} width={48} tick={{ fontSize: 10 }} stroke="#94A3B8" />
          <Tooltip isAnimationActive={false} content={<ComparisonTooltip />} />
          <Legend wrapperStyle={{ fontSize: 10 }} />
          {chart.series.map(([key, name, color]) => <Line key={key} dataKey={key} name={name} type="linear" connectNulls={false} stroke={color} strokeWidth={2} strokeDasharray={key === 'target' ? '5 4' : undefined} dot={<AssessmentDot />} isAnimationActive={false} />)}
        </LineChart>
      </ResponsiveContainer> : <div className="chart-empty">{completed.length ? 'No successful latency samples available.' : 'Comparisons appear as stages finish.'}</div>}</div>
    </article>)}</div>
    <div className="panel stage-table-scroll" role="region" aria-label="Stage comparison table" tabIndex={0}>
      <table className="stage-table"><caption>Stage results · rates in req/s, latency in ms</caption><thead><tr>{['Stage', 'Status', ...(run.analysis ? ['Assessment', 'Evidence'] : []), 'Target', 'Actual starts', 'Completed rate', 'Successful', 'Failed', 'P50', 'P95', 'P99', 'Missed', 'Started', 'Finished'].map(label => <th key={label} scope="col">{label}</th>)}</tr></thead><tbody>{stages.map(stage => {
        const s = stage.summary
        const assessment = assessments.get(stage.number)
        const latency = key => s?.successes > 0 ? formatMetric(s[key], 2) : 'N/A'
        return <tr key={stage.number} data-assessment={assessment?.status}><th scope="row">{stage.number}</th><td>{stage.status}</td>{run.analysis && <><td><AssessmentBadge status={assessment?.status} /></td><td className="stage-evidence"><ul>{assessment?.reasons.map(reason => <li key={reason}>{reason}</li>)}</ul>{assessment?.metrics_available ? <p>Offered: {formatMetric(assessment.achieved_start_percent, 2)}% · Errors: {formatMetric(assessment.error_rate_percent, 2)}% · Eligible: {assessment.eligible_outcomes} · Successes: {assessment.successful_samples}</p> : <p>Valid final metrics unavailable.</p>}</td></>}<td>{stage.target_rps}</td><td>{formatMetric(stage.actual_start_rps, 2)}</td><td>{formatMetric(s?.throughput, 2)}</td><td>{formatMetric(s?.successes)}</td><td>{formatMetric(s?.failures)}</td><td>{latency('p50_latency_ms')}</td><td>{latency('p95_latency_ms')}</td><td>{latency('p99_latency_ms')}</td><td>{formatMetric(s?.missed_schedules)}</td><td>{time(stage.started_at)}</td><td>{time(stage.completed_at)}</td></tr>
      })}</tbody></table>
    </div>
    <p className="timeline-caption">Actual starts = started requests / stage load time. Completed rate = recorded outcomes / total stage time, including draining. Missed schedules can indicate generator limits; they do not establish target-server overload.</p>
  </section>
}
