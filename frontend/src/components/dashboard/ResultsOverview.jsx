import MetricCard from './MetricCard'
import { formatMetric } from './formatMetric'
export default function ResultsOverview({ result }) {
  const latency = key => result?.successes > 0 ? formatMetric(result[key], 2) : 'N/A'
  const metrics = [
    ['Total requests', formatMetric(result?.requests), '', 'All recorded outcomes'],
    ['Successful requests', formatMetric(result?.successes), '', 'Completed HTTP 2xx', 'success'],
    ['Failed requests', formatMetric(result?.failures), '', 'HTTP, network & canceled', result?.failures > 0 ? 'danger' : ''],
    ['Achieved request rate', formatMetric(result?.throughput, 2), 'req/s', 'Recorded requests / total time'],
    ['Average latency', latency('avg_latency_ms'), 'ms', 'Successful responses only'],
    ['P50 latency', latency('p50_latency_ms'), 'ms', '50% completed at or below'],
    ['P95 latency', latency('p95_latency_ms'), 'ms', '95% completed at or below'],
    ['P99 latency', latency('p99_latency_ms'), 'ms', '99% completed at or below'],
  ]
  return <div className="metrics-grid">{metrics.map(([label, value, unit, description, tone]) => <MetricCard key={label} label={label} value={value} unit={unit} description={description} tone={tone} empty={!result} />)}</div>
}
