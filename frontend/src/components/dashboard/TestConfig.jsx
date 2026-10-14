import { useState } from 'react'
import Icon from '../ui/Icon'
import { stagePlan } from './stagePlan'

export default function TestConfig({ onSubmit, loading, loadingLabel, error, initialConfig }) {
  const [mode, setMode] = useState(initialConfig?.mode ?? 'standard')
  const [startRPS, setStartRPS] = useState(initialConfig?.start_rps ?? 10)
  const [increment, setIncrement] = useState(initialConfig?.rps_increment ?? 10)
  const [maxRPS, setMaxRPS] = useState(initialConfig?.max_rps ?? 50)
  const [stageDuration, setStageDuration] = useState(initialConfig?.stage_duration_seconds ?? 5)
  const [url, setUrl] = useState(initialConfig?.url ?? 'http://127.0.0.1:8081')
  const [workers, setWorkers] = useState(initialConfig?.workers ?? 2)
  const [rps, setRps] = useState(initialConfig?.mode === 'stepped' ? 20 : initialConfig?.rps ?? 20)
  const [duration, setDuration] = useState(initialConfig?.mode === 'stepped' ? 5 : initialConfig?.duration_seconds ?? 5)
  const [authorized, setAuthorized] = useState(initialConfig?.authorized ?? false)
  const stepped = mode === 'stepped'
  const plan = stagePlan(Number(startRPS), Number(increment), Number(maxRPS), Number(stageDuration))
  function handleSubmit(event) {
    event.preventDefault()
    if (loading || !authorized) return
    if (stepped && plan.error) return
    const common = { url: url.trim(), method: 'GET', workers: Number(workers), authorized, mode }
    onSubmit(stepped
      ? { ...common, start_rps: Number(startRPS), rps_increment: Number(increment), max_rps: Number(maxRPS), stage_duration_seconds: Number(stageDuration) }
      : { ...common, rps: Number(rps), duration_seconds: Number(duration) })
  }
  return (
    <section className="panel config-panel" aria-labelledby="config-title">
      <div className="panel-heading"><span className="section-number">01</span><div><h2 id="config-title">Configure test</h2><p>A little traffic. A lot of clarity.</p></div></div>
      <form onSubmit={handleSubmit}>
        <fieldset disabled={loading}>
          <legend className="sr-only">HTTP test configuration</legend>
          <div className="field"><label htmlFor="test-mode">Test mode</label><select id="test-mode" value={mode} onChange={e => setMode(e.target.value)}><option value="standard">Standard Test</option><option value="stepped">Stepped Load Test</option></select><p>{stepped ? 'Compare sequential load levels. Each stage drains before the next starts.' : 'Run one fixed target request rate.'}</p></div>
          <div className="field"><label htmlFor="target-url">Target URL</label><input id="target-url" type="url" value={url} onChange={e => setUrl(e.target.value)} required aria-describedby="target-help" spellCheck="false" /><p id="target-help">HTTP only. Use localhost or a loopback IP address.</p></div>
          <div className="field"><label htmlFor="method">HTTP method</label><select id="method" value="GET" disabled aria-describedby="method-help"><option>GET</option></select><p id="method-help">GET is the only supported method.</p></div>
          <div className="field-divider" />
          <div className="field"><label htmlFor="workers">Concurrent workers<span>01–10</span></label><div className="input-unit"><input id="workers" type="number" min="1" max="10" step="1" value={workers} onChange={e => setWorkers(e.target.value)} required aria-describedby="workers-help" /><span>workers</span></div><p id="workers-help">Parallel workers sending requests to your API.</p></div>
          {stepped ? <>
            {[['start-rps', 'Starting target RPS', startRPS, setStartRPS], ['rps-increment', 'RPS step size', increment, setIncrement], ['max-rps', 'Maximum target RPS', maxRPS, setMaxRPS]].map(([id, label, value, setter]) => <div className="field" key={id}><label htmlFor={id}>{label}</label><div className="input-unit"><input id={id} type="number" min="1" max="100" step="1" required value={value} onChange={e => setter(e.target.value)} aria-describedby="stage-preview" /><span>req/s</span></div></div>)}
            <div className="field"><label htmlFor="stage-duration">Duration per stage</label><div className="input-unit"><input id="stage-duration" type="number" min="1" max="30" step="1" required value={stageDuration} onChange={e => setStageDuration(e.target.value)} aria-describedby="stage-preview" /><span>seconds</span></div></div>
            <p id="stage-preview" className={`stage-preview ${plan.error ? 'error' : ''}`} role="status">{plan.error || `${plan.rates.length} stages: ${plan.rates.join(' → ')} req/s. ${plan.totalSeconds} seconds of load, plus draining. Maximum RPS is a ceiling; it is not added as an extra step.`}</p>
          </> : <>
            <div className="field"><label htmlFor="rps">Target request rate<span>01–100</span></label><div className="input-unit"><input id="rps" type="number" min="1" max="100" step="1" value={rps} onChange={e => setRps(e.target.value)} required aria-describedby="rps-help" /><span>req/s</span></div><p id="rps-help">Requests per second across all workers. Actual rate may be lower.</p></div>
            <div className="field"><label htmlFor="duration">Test duration<span>01–30</span></label><div className="input-unit"><input id="duration" type="number" min="1" max="30" step="1" value={duration} onChange={e => setDuration(e.target.value)} required aria-describedby="duration-help" /><span>seconds</span></div><p id="duration-help">How long to schedule requests. In-flight requests may finish afterward.</p></div>
          </>}
          <label className="authorization"><input type="checkbox" checked={authorized} onChange={e => setAuthorized(e.target.checked)} required /><span>I own this API or have permission to load test it.</span></label>
          <button className="run-button" type="submit" disabled={loading || !authorized || (stepped && Boolean(plan.error))}><Icon name={loading ? 'activity' : 'play'} size={18} />{loading ? loadingLabel : 'Run test'}{!loading && <span aria-hidden="true">↗</span>}</button>
        </fieldset>
        <div className="form-feedback">{error ? <p className="error" role="alert">{error}</p> : <p>{loading ? 'One test at a time. Active tests survive page refresh.' : authorized ? 'Ready to test your local API.' : 'Confirm authorization to enable testing.'}</p>}</div>
      </form>
      <div className="local-banner"><Icon name="shield" size={18} /><span>Local targets only <span>·</span> No external hosts</span></div>
    </section>
  )
}
