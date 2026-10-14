import { formatMetric } from './formatMetric'

export default function LiveProgress({ run, active, stopping, onStop }) {
  const p = run.progress
  const percent = Math.min(100, Math.max(0, p.load_elapsed_seconds / p.duration_seconds * 100)) || 0
  return (
    <section className="panel live-panel" aria-label="Test progress">
      <div className="live-heading"><div><h2>{active ? 'Live progress' : 'Execution progress'}</h2><p>Measured load time: {formatMetric(p.load_elapsed_seconds, 1)} / {formatMetric(p.duration_seconds)} s</p></div>{active && <button className="stop-button" onClick={onStop} disabled={stopping || run.cancellation_requested}>{stopping || run.cancellation_requested ? 'Stopping…' : 'Stop test'}</button>}</div>
      <progress aria-label="Measured load duration" value={percent} max="100" />
      {active && <>
        <dl className="live-stats">{[['Started', p.started_requests], ['Recorded outcomes', p.completed_requests], ['Successful', p.successes], ['Failed', p.failures], ['Missed schedules', p.missed_schedules]].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{formatMetric(value)}</dd></div>)}</dl>
        <div className="live-rates"><p><strong>{formatMetric(p.observed_start_rps, 2)} req/s</strong><span>Starts / measured load time</span></p><p><strong>{p.sample_interval_seconds > 0 ? `${formatMetric(p.recent_throughput, 2)} req/s` : 'N/A'}</strong><span>Recorded outcomes / last {formatMetric(p.sample_interval_seconds, 2)} s sample</span></p></div>
      </>}
      <p className="details-note">{active ? 'Updates are measured by the engine about once per second. Latency percentiles and the full report appear after shutdown. You can refresh and recover an active test.' : 'The bar shows measured load duration; a canceled run may finish below 100%.'}</p>
    </section>
  )
}
