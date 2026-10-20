import { lazy, Suspense, useEffect, useState } from 'react'
import { getHistory } from '../../services/testService'
import { compareRuns } from './compare'
import { formatMetric } from '../dashboard/formatMetric'

const ComparisonCharts = lazy(() => import('./ComparisonCharts'))
const display = value => value === null || value === undefined ? 'N/A' : String(value)
const signed = value => Number.isFinite(value) ? `${value > 0 ? '+' : ''}${formatMetric(value, 2)}` : 'N/A'
const level = item => item ? `${item.target_rps} target req/s (stage ${item.stage_number})` : 'None observed'

function MetricTable({ rows, label }) {
  return <div className="panel stage-table-scroll" role="region" aria-label={label} tabIndex={0}><table className="stage-table"><caption>{label} · differences are Candidate − Baseline</caption><thead><tr>{['Metric', 'Baseline', 'Candidate', 'Absolute change', 'Relative change', 'Interpretation'].map(name => <th scope="col" key={name}>{name}</th>)}</tr></thead><tbody>{rows.map(row => <tr key={row.label}><th scope="row">{row.label} ({row.unit})</th><td>{formatMetric(row.baseline, 2)}</td><td>{formatMetric(row.candidate, 2)}</td><td>{signed(row.difference)}{row.unit === '%' && row.difference !== null ? ' pp' : ''}</td><td>{row.percent === null ? 'N/A' : `${signed(row.percent)}%`}</td><td className={row.interpretation.includes('(unfavorable)') ? 'comparison-unfavorable' : row.interpretation.includes('(favorable)') ? 'comparison-favorable' : ''}>{row.interpretation}</td></tr>)}</tbody></table></div>
}

export function ComparisonView({ baseline, candidate, onSwap }) {
  const comparison = compareRuns(baseline, candidate)
  const config = comparison.compatibility
  return <section aria-label="Run comparison">
    <div className="history-toolbar"><a href="#history">← Back to history</a><button onClick={onSwap}>Swap Baseline and Candidate</button></div>
    <h2>Compare Runs</h2>
    <div className="comparison-identities">{[['Baseline', baseline], ['Candidate', candidate]].map(([label, run]) => <article className="panel saved-run-heading" key={label}><h3>{label}</h3><p><a href={`#history/${run.id}`}>{run.id}</a></p><p>{run.config?.url || 'Target unavailable'}</p><p>{run.started_at ? new Date(run.started_at).toLocaleString() : 'Unknown date'} · {run.status}</p></article>)}</div>
    <div className={`panel comparison-warning ${config.status}`} role="status"><h3>{config.label}</h3>{config.reasons.length > 0 && <ul>{config.reasons.map(reason => <li key={reason}>{reason}</li>)}</ul>}<p>Matching configuration does not prove identical machine, network, target state, or environmental conditions.</p></div>
    <details className="panel comparison-config"><summary>Configuration checks</summary><div className="stage-table-scroll"><table className="stage-table"><thead><tr><th scope="col">Setting</th><th scope="col">Baseline</th><th scope="col">Candidate</th><th scope="col">Check</th></tr></thead><tbody>{config.rows.map(row => <tr key={row.label}><th scope="row">{row.label}</th><td>{display(row.baseline)}</td><td>{display(row.candidate)}</td><td>{row.missing ? 'Unavailable' : row.same ? 'Match' : 'Different'}</td></tr>)}</tbody></table></div></details>
    <Suspense fallback={<p className="panel history-empty" role="status">Loading comparison charts…</p>}><ComparisonCharts comparison={comparison} /></Suspense>
    {comparison.standard && <MetricTable rows={comparison.metrics} label="Standard test metrics" />}
    {comparison.stepped && <>
      <p className="timeline-caption">Only common target-RPS stages are paired. Baseline-only levels: {comparison.stages.baselineOnly.join(', ') || 'none'}. Candidate-only levels: {comparison.stages.candidateOnly.join(', ') || 'none'}.</p>
      {!comparison.stages.common.length && <p className="panel history-empty">No unambiguous matching target-RPS stages are available.</p>}
      {comparison.stages.common.map(stage => <section className="comparison-stage" key={stage.target}><h3>{stage.target} target req/s · Baseline {stage.baselineStatus}, Candidate {stage.candidateStatus}</h3>{!stage.conclusions && <p className="timeline-caption">Configuration, completion, or offered-load conditions limit this stage to inspection only.</p>}<MetricTable rows={[stage.p95, stage.starts]} label={`Stage comparison at ${stage.target} target req/s`} /></section>)}
    </>}
    {!comparison.standard && !comparison.stepped && <p className="panel history-empty">These test modes do not share a comparison view. Inspect each saved report using its link above.</p>}
    <section className="panel breakpoint-comparison" aria-label="Breakpoint assessment comparison"><h3>Breakpoint observations</h3>{comparison.breakpoints.baseline && comparison.breakpoints.candidate ? <>
      <div className="stage-table-scroll"><table className="stage-table"><thead><tr><th scope="col">Observation</th><th scope="col">Baseline</th><th scope="col">Candidate</th></tr></thead><tbody>
        <tr><th scope="row">Highest passing tested level</th><td>{level(baseline.analysis.highest_passing_stage)}</td><td>{level(candidate.analysis.highest_passing_stage)}</td></tr>
        <tr><th scope="row">First failing tested level</th><td>{level(baseline.analysis.first_failing_stage)}</td><td>{level(candidate.analysis.first_failing_stage)}</td></tr>
        <tr><th scope="row">Breakpoint status</th><td>{baseline.analysis.breakpoint.status.replaceAll('_', ' ')}</td><td>{candidate.analysis.breakpoint.status.replaceAll('_', ' ')}</td></tr>
      </tbody></table></div><p>Baseline: {baseline.analysis.breakpoint.message}</p><p>Candidate: {candidate.analysis.breakpoint.message}</p>
    </> : <p>Both runs need saved breakpoint assessments for this section. No missing assessment is treated as a pass.</p>}<p>A higher passing tested level does not establish an increase in sustainable capacity.</p></section>
    <p className="timeline-caption">Relative change is (Candidate − Baseline) / Baseline × 100; zero or unavailable baselines produce N/A. Error-rate absolute changes use percentage points (pp). HTTP error rate = HTTP errors / (successes + HTTP errors), excluding network/cancellation outcomes. Latencies describe successes only; inspect them alongside failures.</p>
    <p className="timeline-caption">Directional labels require matching completed configurations and comparable offered load: each start rate must reach 95–105% of target and differ by no more than 5 percentage points, with no cancellation outcomes. Counts and start-rate changes are neutral. A higher failure fraction suppresses throughput favorability. These are descriptive differences, not statistical significance tests.</p>
    <p className="timeline-caption">Throughput samples align at their actual elapsed interval ends, never wall-clock dates. Straight segments connect adjacent measurements; missing timestamps/windows remain gaps, without synthetic points or interpolation.</p>
  </section>
}

export default function CompareRuns({ baselineId, candidateId, onLoaded, onSwap }) {
  const [state, setState] = useState({ loading: true, error: '', baseline: null, candidate: null })
  const [reload, setReload] = useState(0)
  useEffect(() => {
    const controller = new AbortController()
    Promise.all([getHistory(baselineId, controller.signal), getHistory(candidateId, controller.signal)]).then(([baseline, candidate]) => {
      if (!controller.signal.aborted) { setState({ loading: false, error: '', baseline, candidate }); onLoaded([baseline, candidate]) }
    }).catch(err => { if (!controller.signal.aborted) setState({ loading: false, error: err.status === 404 ? 'A selected report no longer exists. Return to history and select another run.' : err.message, baseline: null, candidate: null }) })
    return () => controller.abort()
  }, [baselineId, candidateId, onLoaded, reload])
  if (state.loading) return <p className="panel history-empty" role="status">Loading Baseline and Candidate…</p>
  if (state.error) return <section className="panel history-empty"><p role="alert" className="error">{state.error}</p><button onClick={() => { setState(current => ({ ...current, loading: true })); setReload(value => value + 1) }}>Try again</button><a href="#history">Back to history</a></section>
  return <ComparisonView baseline={state.baseline} candidate={state.candidate} onSwap={onSwap} />
}
