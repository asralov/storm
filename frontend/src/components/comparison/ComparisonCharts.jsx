import { Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

const baselineColor = '#60A5FA'
const candidateColor = '#FBBF24'
const axis = { stroke: '#94A3B8', tick: { fontSize: 10 }, tickLine: false, axisLine: false }
const tooltip = { background: '#111827', border: '1px solid #253047', borderRadius: 6 }

function CandidatePoint({ cx, cy }) {
  return Number.isFinite(cx) && Number.isFinite(cy) ? <path d={`M${cx},${cy - 4}l4,4 -4,4 -4,-4Z`} fill={candidateColor} /> : null
}

function Lines({ data, xKey, xUnit, unit, title }) {
  if (!data.some(point => Number.isFinite(point.baseline) || Number.isFinite(point.candidate))) return <article className="panel chart-panel"><h3>{title}</h3><p className="chart-empty">No matching measured values are available.</p></article>
  return <article className="panel chart-panel" aria-label={title}><h3>{title}</h3><div className="chart-canvas"><ResponsiveContainer width="100%" height="100%" minWidth={0}>
    <LineChart data={data} accessibilityLayer title={title} margin={{ top: 15, right: 18, bottom: 5, left: 0 }}>
      <CartesianGrid stroke="#253047" vertical={false} strokeDasharray="3 5" />
      <XAxis {...axis} dataKey={xKey} type="number" domain={[0, 'dataMax']} tickFormatter={v => `${Number(v.toFixed(2))}${xUnit}`} />
      <YAxis {...axis} width={48} domain={[0, 'auto']} />
      <Tooltip contentStyle={tooltip} labelFormatter={v => `${Number(v).toFixed(2)} ${xUnit}`} formatter={(value, name) => [`${Number(value).toFixed(2)} ${unit}`, name]} isAnimationActive={false} />
      <Legend wrapperStyle={{ fontSize: 11 }} />
      <Line dataKey="baseline" name="Baseline (solid, circles)" stroke={baselineColor} strokeWidth={2} type="linear" dot={{ r: 3 }} connectNulls={false} isAnimationActive={false} />
      <Line dataKey="candidate" name="Candidate (dashed, diamonds)" stroke={candidateColor} strokeWidth={2} strokeDasharray="5 4" type="linear" dot={<CandidatePoint />} connectNulls={false} isAnimationActive={false} />
    </LineChart>
  </ResponsiveContainer></div></article>
}

export default function ComparisonCharts({ comparison }) {
  const usable = points => points.some(p => Number.isFinite(p.baseline) || Number.isFinite(p.candidate))
  const stagePoints = comparison.stages.common.filter(stage => stage.baselineStatus === 'completed' && stage.candidateStatus === 'completed')
  return <div className="charts-grid comparison-charts">
    {comparison.standard && <>
      <article className="panel chart-panel" aria-label="Saved latency percentiles"><h3>Saved latency percentiles · ms</h3><div className="chart-canvas">{usable(comparison.latencyBars) ? <ResponsiveContainer width="100%" height="100%" minWidth={0}>
        <BarChart data={comparison.latencyBars} accessibilityLayer title="Baseline and Candidate latency percentiles" margin={{ top: 15, right: 18, bottom: 5, left: 0 }}>
          <CartesianGrid stroke="#253047" vertical={false} strokeDasharray="3 5" /><XAxis {...axis} dataKey="label" /><YAxis {...axis} width={48} domain={[0, 'auto']} />
          <Tooltip contentStyle={tooltip} formatter={(value, name) => [`${Number(value).toFixed(2)} ms`, name]} isAnimationActive={false} /><Legend wrapperStyle={{ fontSize: 11 }} />
          <Bar dataKey="baseline" name="Baseline (left)" fill={baselineColor} isAnimationActive={false} /><Bar dataKey="candidate" name="Candidate (right)" fill={candidateColor} isAnimationActive={false} />
        </BarChart>
      </ResponsiveContainer> : <div className="chart-empty">No successful-response percentile measurements available.</div>}</div></article>
      {usable(comparison.throughput) ? <Lines title="Completed throughput over elapsed time" data={comparison.throughput} xKey="elapsed" xUnit="s" unit="req/s" /> : <div className="panel history-empty">Throughput overlay requires comparable configurations and saved interval samples.</div>}
    </>}
    {comparison.stepped && stagePoints.length > 0 && <>
      <Lines title="P95 at common target levels · ms" data={stagePoints.map(s => ({ target: s.target, baseline: s.p95.baseline, candidate: s.p95.candidate }))} xKey="target" xUnit="req/s target" unit="ms" />
      <Lines title="Actual starts at common target levels · req/s" data={stagePoints.map(s => ({ target: s.target, baseline: s.starts.baseline, candidate: s.starts.candidate }))} xKey="target" xUnit="req/s target" unit="req/s" />
    </>}
  </div>
}
