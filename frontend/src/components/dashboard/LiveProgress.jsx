import { formatMetric } from './formatMetric'

export default function LiveProgress({ run, active, stopping, onStop }) {
  const p = run.progress
  const stepped = run.config.mode === 'stepped'
  const percent = Math.min(100, Math.max(0, (stepped ? run.overall_load_seconds / run.planned_load_seconds : p.load_elapsed_seconds / p.duration_seconds) * 100)) || 0
  return (
    <section className="panel live-panel" aria-label="Test progress">
      <div className="live-heading"><div><h2>{stepped ? `Stage ${run.current_stage || 0} of ${run.total_stages} · ${p.target_rps} req/s target` : active ? 'Live progress' : 'Execution progress'}</h2><p>{stepped ? 'Stage load time' : 'Measured load time'}: {formatMetric(p.load_elapsed_seconds, 1)} / {formatMetric(p.duration_seconds)} s</p>{stepped && <p>Overall elapsed: {formatMetric(run.overall_elapsed_seconds, 1)} s · Load: {formatMetric(run.overall_load_seconds, 1)} / {formatMetric(run.planned_load_seconds)} s</p>}</div>{active && <button className="stop-button" onClick={onStop} disabled={stopping || run.cancellation_requested}>{stopping || run.cancellation_requested ? 'Stopping…' : 'Stop test'}</button>}</div>
      <progress aria-label={stepped ? 'Overall measured load duration' : 'Measured load duration'} value={percent} max="100" />
      {stepped && <p className="details-note">Counts and live rates below belong to the current stage. Overall load progress excludes drain time; a full bar does not mean all requests have finished.</p>}
      {active && <>
        <dl className="live-stats">{[['Started', p.started_requests], ['Recorded outcomes', p.completed_requests], ['Successful', p.successes], ['Failed', p.failures], ['Missed schedules', p.missed_schedules]].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{formatMetric(value)}</dd></div>)}</dl>
        <div className="live-rates"><p><strong>{formatMetric(p.observed_start_rps, 2)} req/s</strong><span>Starts / measured load time</span></p><p><strong>{p.sample_interval_seconds > 0 ? `${formatMetric(p.recent_throughput, 2)} req/s` : 'N/A'}</strong><span>Recorded outcomes / last {formatMetric(p.sample_interval_seconds, 2)} s sample</span></p></div>
      </>}
      <p className="details-note">{active ? 'Updates are measured about once per second. Interval latency appears in the charts; full-run percentiles appear after shutdown. Refresh recovers active runs and retained windows.' : 'The bar shows measured load duration; a canceled run may finish below 100%.'}</p>
    </section>
  )
}
