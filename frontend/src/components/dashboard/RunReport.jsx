import { lazy, Suspense, useState } from 'react'
import ResultsOverview from './ResultsOverview'
import ResultsDetails from './ResultsDetails'
import LiveProgress from './LiveProgress'
import { mergeIntervals } from '../../hooks/timeline'

const LiveCharts = lazy(() => import('./LiveCharts'))
const SteppedResults = lazy(() => import('./SteppedResults'))

export function PersistenceNotice({ run }) {
  const state = run?.persistence
  if (!state) return null
  if (state.error) return <p className="persistence-notice error" role="alert">{state.error}</p>
  if (state.status === 'saving') return <p className="persistence-notice" role="status">Execution finished. Saving the report locally…</p>
  if (state.status === 'deleted') return <p className="persistence-notice">This report was deleted from history. It remains visible in this session.</p>
  if (state.status === 'saved') return <p className="persistence-notice">Saved locally. <a href={`#history/${run.id}`}>View saved report</a></p>
  return null
}

export default function RunReport({ run, active = false, stopping = false, onStop, starting = false, readOnly = false }) {
  const [selected, setSelected] = useState('')
  const stepped = run?.config.mode === 'stepped'
  const stage = readOnly && stepped ? run.stages?.find(item => item.number === Number(selected || run.current_stage)) ?? run.stages?.at(-1) : null
  const samples = readOnly ? mergeIntervals([], stage?.intervals ?? run?.intervals) : starting ? [] : run?.timeline
  if (run?.status === 'interrupted') return <div className="panel history-empty"><h2>Interrupted execution</h2><p>{run.error}</p><p>No final metrics or completion timestamp were recovered. This run was not resumed.</p></div>
  return <>
    {!readOnly && <PersistenceNotice run={run} />}
    {run && !readOnly && <LiveProgress run={run} active={active && run.persistence?.status !== 'saving'} stopping={stopping} onStop={onStop} />}
    <Suspense fallback={<div className="panel chart-empty" role="status">Loading report charts…</div>}>
      {stepped && <SteppedResults run={run} />}
      {stepped && <h2 className="stage-timeline-title">Stage {stage?.number ?? run.current_stage ?? 0} timeline</h2>}
      {readOnly && stepped && <div className="field history-stage-picker"><label htmlFor="saved-stage">Saved stage timeline</label><select id="saved-stage" value={stage?.number ?? ''} onChange={e => setSelected(e.target.value)}>{run.stages.map(item => <option key={item.number} value={item.number}>Stage {item.number} · {item.target_rps} req/s · {item.status}</option>)}</select></div>}
      <LiveCharts key={`${run?.id || 'new'}-${stage?.number ?? run?.current_stage ?? 0}`} samples={samples} recovered={!readOnly && run?.chartRecovered} archived={readOnly} targetRPS={stepped ? stage?.target_rps ?? run.progress.target_rps : run?.config.rps} />
    </Suspense>
    {!stepped && <><ResultsOverview result={run?.summary} /><ResultsDetails result={run?.summary} /></>}
  </>
}
