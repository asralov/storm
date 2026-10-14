import { useState } from 'react'
import { CartesianGrid, Line, LineChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { formatMetric } from './formatMetric'

const charts = [
  { key: 'throughput', title: 'Completed throughput', unit: 'req/s', description: 'Recorded outcomes / actual window duration, including failures.', series: [{ key: 'throughput', label: 'Completed', color: '#60A5FA' }] },
  { key: 'latency', title: 'Interval latency', unit: 'ms', description: 'Successful HTTP 2xx responses completed in each measurement window.', series: [
    { key: 'p50', label: 'P50', color: '#60A5FA' }, { key: 'p95', label: 'P95', color: '#34D399', dash: '6 3' },
    { key: 'p99', label: 'P99', color: '#FBBF24', dash: '2 3' }, { key: 'average', label: 'Average', color: '#C4B5FD', dash: '8 3 2 3' },
  ] },
  { key: 'failures', title: 'Failures per interval', unit: 'requests', description: 'Failures in each window. A final partial window may be shorter.', series: [
    { key: 'failures', label: 'Total', color: '#F87171' }, { key: 'httpErrors', label: 'HTTP', color: '#FBBF24', dash: '6 3' },
    { key: 'networkErrors', label: 'Network', color: '#C4B5FD', dash: '2 3' }, { key: 'canceled', label: 'Canceled', color: '#94A3B8', dash: '8 3 2 3' },
  ] },
]

function exact(value) {
  return Number.isFinite(value) ? value.toLocaleString('en-US', { maximumFractionDigits: 6 }) : 'N/A'
}

function SampleTooltip({ active, payload, unit }) {
  if (!active || !payload?.length) return null
  const point = payload[0].payload
  return <div className="chart-tooltip"><p>{exact(point.start)}–{exact(point.elapsed)} s · {exact(point.duration)} s window</p>{payload.map(series => <p key={series.dataKey}><strong style={{ color: series.color }}>{series.name}: {exact(series.value)} {unit}</strong></p>)}<p>{point.successes} successful / {point.completed} recorded outcomes</p>{point.successes > 0 && point.successes < 20 && <p>Small sample: tail estimates are limited.</p>}</div>
}

function TimelineChart({ chart, samples, targetRPS }) {
  const [hidden, setHidden] = useState([])
  const visible = chart.series.filter(series => !hidden.includes(series.key))
  const valid = samples.some(point => visible.some(series => Number.isFinite(point[series.key])))
  function toggle(key) { setHidden(current => current.includes(key) ? current.filter(value => value !== key) : [...current, key]) }
  return <article className="panel chart-panel" aria-label={`${chart.title} chart`}>
    <div className="chart-heading"><h3>{chart.title}</h3><span>{chart.unit}</span></div>
    <p className="chart-description">{chart.description}</p>
    <div className="chart-legend" aria-label={`${chart.title} series`}>{chart.series.map(series => <button type="button" key={series.key} aria-pressed={!hidden.includes(series.key)} onClick={() => toggle(series.key)}><span style={{ background: series.color }} />{series.label}</button>)}</div>
    <div className="chart-canvas">{valid ? <ResponsiveContainer width="100%" height="100%" minWidth={0}>
      <LineChart data={samples} margin={{ top: 24, right: 18, bottom: 5, left: 0 }} accessibilityLayer title={`${chart.title} over elapsed seconds`}>
        <CartesianGrid stroke="#253047" strokeDasharray="3 5" vertical={false} />
        <XAxis dataKey="elapsed" type="number" domain={[0, 'dataMax']} tickFormatter={v => `${formatMetric(v, 1)}s`} stroke="#94A3B8" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} minTickGap={25} />
        <YAxis width={48} domain={[0, 'auto']} allowDecimals={chart.key !== 'failures'} tickFormatter={v => formatMetric(v, Number.isInteger(v) ? 0 : 1)} stroke="#94A3B8" tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
        <Tooltip content={<SampleTooltip unit={chart.unit} />} isAnimationActive={false} cursor={{ stroke: '#94A3B8', strokeDasharray: '3 3' }} />
        {chart.key === 'throughput' && Number.isFinite(targetRPS) && <ReferenceLine y={targetRPS} ifOverflow="extendDomain" stroke="#FBBF24" strokeDasharray="5 4" label={{ value: `Start target: ${targetRPS} req/s`, position: 'insideTopRight', fill: '#FBBF24', fontSize: 10 }} />}
        {visible.map(series => <Line key={series.key} dataKey={series.key} name={series.label} type="linear" stroke={series.color} strokeDasharray={series.dash} strokeWidth={2} dot={{ r: 2, strokeWidth: 0, fill: series.color }} activeDot={{ r: 4 }} connectNulls={false} isAnimationActive={false} />)}
      </LineChart>
    </ResponsiveContainer> : <div className="chart-empty">{!visible.length ? 'Select a series above to display it.' : !samples.length ? 'Waiting for the first completed measurement window.' : chart.key === 'latency' ? 'No successful-response latency samples in these windows.' : 'No measurements available.'}</div>}</div>
    <p className="chart-latest">{samples.length ? `Latest window ends at ${formatMetric(samples.at(-1).elapsed, 2)} s · ${samples.at(-1).successes} successful responses` : 'No measurement available'}{chart.key === 'throughput' && ' · The start target does not guarantee completed throughput.'}</p>
  </article>
}

export default function LiveCharts({ samples = [], recovered = false, targetRPS }) {
  const truncated = samples.length > 0 && samples[0].sequence > 1
  return <section className="timeline-section" aria-labelledby="timeline-title">
    <div className="details-heading"><h2 id="timeline-title">Performance timeline</h2><span className="eyebrow">{samples.length} WINDOWS</span></div>
    <p className="timeline-caption">{recovered ? 'Recovered measured interval history from the backend.' : 'One-second measurement windows, plus the final partial window.'} Time includes draining.{truncated && ' Earlier windows are outside the retained history.'}</p>
    <div className="charts-grid">{charts.map(chart => <TimelineChart key={chart.key} chart={chart} samples={samples} targetRPS={targetRPS} />)}</div>
    <p className="timeline-caption">Charts use interval statistics; summary cards use the whole run. Nearest-rank percentiles with very few successes have limited statistical meaning. Up to 300 measured windows are retained. Lines connect measurements without smoothing.</p>
  </section>
}
