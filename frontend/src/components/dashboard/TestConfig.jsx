import { useState } from 'react'
import Icon from '../ui/Icon'

export default function TestConfig({ onSubmit, loading, loadingLabel, error, initialConfig }) {
  const [url, setUrl] = useState(initialConfig?.url ?? 'http://127.0.0.1:8081')
  const [workers, setWorkers] = useState(initialConfig?.workers ?? 2)
  const [rps, setRps] = useState(initialConfig?.rps ?? 20)
  const [duration, setDuration] = useState(initialConfig?.duration_seconds ?? 5)
  const [authorized, setAuthorized] = useState(initialConfig?.authorized ?? false)
  function handleSubmit(event) {
    event.preventDefault()
    if (loading || !authorized) return
    onSubmit({ url: url.trim(), method: 'GET', workers: Number(workers), rps: Number(rps), duration_seconds: Number(duration), authorized })
  }
  return (
    <section className="panel config-panel" aria-labelledby="config-title">
      <div className="panel-heading"><span className="section-number">01</span><div><h2 id="config-title">Configure test</h2><p>A little traffic. A lot of clarity.</p></div></div>
      <form onSubmit={handleSubmit}>
        <fieldset disabled={loading}>
          <legend className="sr-only">HTTP test configuration</legend>
          <div className="field"><label htmlFor="target-url">Target URL</label><input id="target-url" type="url" value={url} onChange={e => setUrl(e.target.value)} required aria-describedby="target-help" spellCheck="false" /><p id="target-help">HTTP only. Use localhost or a loopback IP address.</p></div>
          <div className="field"><label htmlFor="method">HTTP method</label><select id="method" value="GET" disabled aria-describedby="method-help"><option>GET</option></select><p id="method-help">GET is the only supported method.</p></div>
          <div className="field-divider" />
          <div className="field"><label htmlFor="workers">Concurrent workers<span>01–10</span></label><div className="input-unit"><input id="workers" type="number" min="1" max="10" step="1" value={workers} onChange={e => setWorkers(e.target.value)} required aria-describedby="workers-help" /><span>workers</span></div><p id="workers-help">Parallel workers sending requests to your API.</p></div>
          <div className="field"><label htmlFor="rps">Target request rate<span>01–100</span></label><div className="input-unit"><input id="rps" type="number" min="1" max="100" step="1" value={rps} onChange={e => setRps(e.target.value)} required aria-describedby="rps-help" /><span>req/s</span></div><p id="rps-help">Requests per second across all workers. Actual rate may be lower.</p></div>
          <div className="field"><label htmlFor="duration">Test duration<span>01–30</span></label><div className="input-unit"><input id="duration" type="number" min="1" max="30" step="1" value={duration} onChange={e => setDuration(e.target.value)} required aria-describedby="duration-help" /><span>seconds</span></div><p id="duration-help">How long to schedule requests. In-flight requests may finish afterward.</p></div>
          <label className="authorization"><input type="checkbox" checked={authorized} onChange={e => setAuthorized(e.target.checked)} required /><span>I own this API or have permission to load test it.</span></label>
          <button className="run-button" type="submit" disabled={loading || !authorized}><Icon name={loading ? 'activity' : 'play'} size={18} />{loading ? loadingLabel : 'Run test'}{!loading && <span aria-hidden="true">↗</span>}</button>
        </fieldset>
        <div className="form-feedback">{error ? <p className="error" role="alert">{error}</p> : <p>{loading ? 'One test at a time. Active tests survive page refresh.' : authorized ? 'Ready to test your local API.' : 'Confirm authorization to enable testing.'}</p>}</div>
      </form>
      <div className="local-banner"><Icon name="shield" size={18} /><span>Local targets only <span>·</span> No external hosts</span></div>
    </section>
  )
}
