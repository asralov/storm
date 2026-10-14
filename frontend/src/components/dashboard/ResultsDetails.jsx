import { formatMetric } from './formatMetric'
export default function ResultsDetails({ result }) {
  const groups = [
    ['Response breakdown', [['HTTP errors', 'http_error_count'], ['Network errors', 'network_errors'], ['Canceled requests', 'canceled']]],
    ['Request scheduling', [['Scheduled requests', 'scheduled_requests'], ['Dispatched requests', 'dispatched_requests'], ['Started requests', 'started_requests'], ['Missed scheduling opportunities', 'missed_schedules']]],
  ]
  return (
    <section className="panel details-panel" aria-labelledby="details-title">
      <div className="details-heading"><h2 id="details-title">Behind the numbers</h2><span className="eyebrow">EXECUTION DETAILS</span></div>
      <div className="details-grid">{groups.map(([title, rows]) => <div key={title}><h3>{title}</h3><dl>{rows.map(([label, key]) => <div className="detail-row" key={key}><dt>{label}</dt><dd>{result ? formatMetric(result[key]) : '—'}</dd></div>)}</dl></div>)}</div>
      <div className="timing-row">{[['Load window', 'load_window_seconds'], ['Total execution', 'elapsed_seconds']].map(([label, key]) => <div key={key}><span>{label}</span><strong>{result ? `${formatMetric(result[key], 2)}${Number.isFinite(result[key]) ? ' s' : ''}` : '—'}</strong></div>)}</div>
      <p className="details-note">Total execution includes in-flight requests finishing after the load window. Scheduling counts reflect observed scheduler activity.</p>
    </section>
  )
}
