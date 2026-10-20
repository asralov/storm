import { useCallback, useEffect, useState } from 'react'
import { listHistory, getHistory } from '../../services/testService'
import { deleteSavedReport } from '../../services/historyActions'
import RunReport from '../dashboard/RunReport'
import { formatMetric } from '../dashboard/formatMetric'
import CompareRuns from '../comparison/CompareRuns'
import { comparisonRoute, parseComparisonRoute, swapSelection, toggleSelection } from '../comparison/compare'

const date = value => value ? new Date(value).toLocaleString() : 'Unknown'
const percent = value => Number.isFinite(value) ? `${formatMetric(value, 2)}%` : 'N/A'

export function HistoryListView({ loading, error, page, offset = 0, deleting, onRefresh, onPage, onDelete, selection = [], onSelect, onSwap, onClear, onCompare }) {
  return <section aria-label="Saved test history">
    <div className="history-toolbar"><p>Reports stored on this machine, newest first.</p><button onClick={onRefresh} disabled={loading}>Refresh</button></div>
    <div className="panel comparison-selection"><p>Select exactly two runs. First is Baseline; second is Candidate. Selection is kept across pages.</p><ol>{selection.map((item, index) => <li key={item.id}><strong>{index === 0 ? 'Baseline' : 'Candidate'}</strong>: {item.url || item.id} <span>({item.id})</span></li>)}</ol><div className="history-toolbar"><button disabled={selection.length !== 2 || loading || Boolean(deleting)} onClick={onCompare}>Compare Runs</button><button disabled={selection.length !== 2} onClick={onSwap}>Swap selection</button><button disabled={!selection.length} onClick={onClear}>Clear selection</button><span role="status">{selection.length} of 2 selected</span></div></div>
    {error && <p role="alert" className="error history-error">{error}</p>}
    {loading ? <p role="status" className="panel history-empty">Loading saved tests…</p> : !page ? <div className="panel history-empty"><p>History could not be loaded.</p><button onClick={onRefresh}>Try again</button></div> : !page.items.length ? <div className="panel history-empty"><h2>{page.total ? 'No reports on this page' : 'Your test history starts here'}</h2><p>{page.total ? 'Go to the previous page or refresh.' : 'Completed, canceled, and failed tests will appear here after they are saved.'}</p><a href="#dashboard">Go to Dashboard</a></div> : <div className="panel stage-table-scroll" role="region" aria-label="Saved tests table" tabIndex={0}>
      <table className="stage-table history-table"><caption>{page.total} saved {page.total === 1 ? 'report' : 'reports'}</caption><thead><tr>{['Compare', 'Date / target', 'Mode / load', 'Execution (s)', 'Requests', 'Completed (req/s)', 'P95 (ms)', 'HTTP/transport errors', 'Status', 'Breakpoint', 'Actions'].map(label => <th key={label} scope="col">{label}</th>)}</tr></thead><tbody>{page.items.map(item => <tr key={item.id}>
        <td><label><input type="checkbox" aria-label={`Select ${item.id} for comparison`} checked={selection.some(selected => selected.id === item.id)} disabled={Boolean(deleting) || (selection.length === 2 && !selection.some(selected => selected.id === item.id))} onChange={() => onSelect(item)} />{selection[0]?.id === item.id ? ' Baseline' : selection[1]?.id === item.id ? ' Candidate' : ''}</label></td>
        <th scope="row" className="history-target"><a href={`#history/${item.id}`}>{date(item.started_at)}</a><span>{item.url}</span></th>
        <td>{item.mode}<small>{item.configured_load}</small></td><td>{formatMetric(item.duration_seconds, 2)}</td><td>{formatMetric(item.total_requests)}</td><td>{formatMetric(item.achieved_rate, 2)}</td><td>{formatMetric(item.p95_latency_ms, 2)}{item.mode === 'stepped' && <small>{item.p95_label}</small>}</td><td>{percent(item.error_rate_percent)}</td><td>{item.status}</td><td>{item.breakpoint_status?.replaceAll('_', ' ') || '—'}</td>
        <td><a className="history-action" href={`#history/${item.id}`}>View report</a><button className="history-delete" disabled={Boolean(deleting)} onClick={() => onDelete(item)} aria-label={`Delete saved run ${item.id}`}>{deleting === item.id ? 'Deleting…' : 'Delete'}</button></td>
      </tr>)}</tbody></table>
    </div>}
    <div className="history-pagination"><button onClick={() => onPage(Math.max(0, offset - 20))} disabled={loading || offset === 0}>Previous</button><span>Page {Math.floor(offset / 20) + 1}</span><button onClick={() => onPage(offset + 20)} disabled={loading || !page || offset + 20 >= page.total}>Next</button></div>
    <p className="timeline-caption">Stepped rows sum executed-stage requests and time; their latency is the maximum stage P95, not an overall percentile. Error rate excludes cancellations. Open a report for the full context.</p>
  </section>
}

function HistoryList({ offset, setOffset, selection, setSelection }) {
  const [reload, setReload] = useState(0)
  const [state, setState] = useState({ loading: true, page: null, error: '' })
  const [deleting, setDeleting] = useState('')
  useEffect(() => {
    const controller = new AbortController()
    listHistory(offset, controller.signal).then(page => { if (!controller.signal.aborted) setState({ loading: false, page, error: '' }) }).catch(err => { if (!controller.signal.aborted) setState({ loading: false, page: null, error: err.message }) })
    return () => controller.abort()
  }, [offset, reload])
  function refresh() { setState(current => ({ ...current, loading: true, error: '' })); setReload(value => value + 1) }
  function changePage(next) { setState(current => ({ ...current, loading: true, error: '' })); setOffset(next) }
  async function remove(item) {
    setDeleting(item.id)
    try {
      if (!await deleteSavedReport(item)) return
      setSelection(current => current.filter(selected => selected.id !== item.id))
      if (state.page?.items.length === 1 && offset > 0) changePage(Math.max(0, offset - 20))
      else refresh()
    } catch (err) { setState(current => ({ ...current, error: `Could not delete report: ${err.message}` })) }
    finally { setDeleting('') }
  }
  return <HistoryListView {...state} offset={offset} deleting={deleting} onRefresh={refresh} onPage={changePage} onDelete={remove} selection={selection} onSelect={item => setSelection(current => toggleSelection(current, item))} onSwap={() => setSelection(swapSelection)} onClear={() => setSelection([])} onCompare={() => { const route = comparisonRoute(selection); if (route) window.location.hash = route.slice(1) }} />
}

export function SavedRunDetail({ run, error, deleting, onDelete }) {
  return <section aria-label="Saved test report">
    <div className="history-toolbar"><a href="#history">← Back to history</a><button className="history-delete" onClick={onDelete} disabled={deleting}>{deleting ? 'Deleting…' : 'Delete saved report'}</button></div>
    {error && <p role="alert" className="error history-error">{error}</p>}
    <div className="panel saved-run-heading"><div><h2>Saved {run.config.mode === 'stepped' ? 'stepped' : 'standard'} test</h2><span className="badge">{run.status} · Read-only</span></div><p className="saved-target">{run.config.method} {run.config.url}</p><p>{run.config.mode === 'stepped' ? `Start ${run.config.start_rps}, maximum ${run.config.max_rps}, step ${run.config.rps_increment} req/s · ${run.config.stage_duration_seconds} s per stage` : `Target ${run.config.rps} req/s · ${run.config.duration_seconds} s configured load`}</p><p>{run.config.workers} workers · Started {date(run.started_at)} · Finished {date(run.completed_at)}</p><p>Run ID: {run.id}</p></div>
    <RunReport key={run.id} run={run} readOnly />
  </section>
}

function HistoryDetail({ id }) {
  const [state, setState] = useState({ loading: true, run: null, error: '' })
  const [reload, setReload] = useState(0)
  const [deleting, setDeleting] = useState(false)
  useEffect(() => {
    const controller = new AbortController()
    getHistory(id, controller.signal).then(run => { if (!controller.signal.aborted) setState({ loading: false, run, error: '' }) }).catch(err => { if (!controller.signal.aborted) setState({ loading: false, run: null, error: err.message }) })
    return () => controller.abort()
  }, [id, reload])
  async function remove() {
    setDeleting(true)
    try { if (await deleteSavedReport(state.run)) window.location.hash = 'history' }
    catch (err) { setState(current => ({ ...current, error: `Could not delete report: ${err.message}` })) }
    finally { setDeleting(false) }
  }
  if (state.loading) return <p className="panel history-empty" role="status">Loading saved report…</p>
  if (!state.run) return <div className="panel history-empty"><p role="alert" className="error">{state.error}</p><button onClick={() => { setState(current => ({ ...current, loading: true })); setReload(value => value + 1) }}>Try again</button><a href="#history">Back to history</a></div>
  return <SavedRunDetail run={state.run} error={state.error} deleting={deleting} onDelete={remove} />
}

export default function HistoryPage({ id }) {
  const [selection, setSelection] = useState([])
  const [offset, setOffset] = useState(0)
  const restoreSelection = useCallback(reports => setSelection(reports.map(run => ({ id: run.id, url: run.config?.url, mode: run.config?.mode }))), [])
  useEffect(() => { const deleted = event => setSelection(current => current.filter(item => item.id !== event.detail.id)); window.addEventListener('storm-history-deleted', deleted); return () => window.removeEventListener('storm-history-deleted', deleted) }, [])
  if (id?.startsWith('compare/')) {
    const pair = parseComparisonRoute(id)
    if (!pair) return <div className="panel history-empty"><p role="alert">Select two different saved runs to compare.</p><a href="#history">Back to history</a></div>
    return <CompareRuns key={pair.join('/')} baselineId={pair[0]} candidateId={pair[1]} onLoaded={restoreSelection} onSwap={() => { setSelection(swapSelection); window.location.hash = `history/compare/${pair[1]}/${pair[0]}` }} />
  }
  return id ? <HistoryDetail key={id} id={id} /> : <HistoryList offset={offset} setOffset={setOffset} selection={selection} setSelection={setSelection} />
}
