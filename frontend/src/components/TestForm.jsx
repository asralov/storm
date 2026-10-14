import useTestRun from '../hooks/useTestRun'
import TestConfig from './dashboard/TestConfig'
import ResultsOverview from './dashboard/ResultsOverview'
import ResultsDetails from './dashboard/ResultsDetails'
import LiveProgress from './dashboard/LiveProgress'
import Icon from './ui/Icon'

export default function TestForm() {
  const { run, active, recovering, starting, stopping, busy, error, submit, stop } = useTestRun()
  const status = recovering ? 'Checking active run' : starting ? 'Starting' : run?.cancellation_requested && active ? 'Stopping' : run?.status || 'Awaiting test'
  const messages = {
    pending: 'Your test is queued to start.', running: 'Your test is running.',
    draining: 'Scheduling finished. Waiting for in-flight requests.',
    completed: 'Test complete. Here’s how it performed.',
    canceled: 'Test canceled. Results include all requests recorded before shutdown.',
    failed: run?.error || 'The test could not finish.',
  }
  return (
    <div className="dashboard-grid">
      <TestConfig key={run?.id || 'new'} initialConfig={run?.config} onSubmit={submit} loading={busy} loadingLabel={recovering ? 'Checking active run…' : starting ? 'Starting test…' : 'Test active'} error={error} />
      <section className="results-column" aria-labelledby="results-title">
        <div className="section-heading"><div><div className="eyebrow">PERFORMANCE REPORT</div><h2 id="results-title">Results overview</h2></div><span className={`badge ${run?.status === 'completed' ? 'success' : ''}`} role="status">{status}</span></div>
        <div className="result-message" role="status"><span className="message-icon"><Icon name="activity" size={22} /></span><div><strong>{recovering ? 'Looking for an active test.' : starting ? 'Starting your test.' : run ? messages[run.status] : 'Your next insight starts with a test.'}</strong><p>{run ? `${run.config.method} ${run.config.url} · ${run.config.workers} workers · target ${run.config.rps} req/s` : 'Configure a local target and run a test. Your actual results will appear here.'}</p></div></div>
        {run && <LiveProgress run={run} active={active} stopping={stopping} onStop={stop} />}
        <ResultsOverview result={run?.summary} />
        <ResultsDetails result={run?.summary} />
      </section>
    </div>
  )
}
