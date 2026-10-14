export default function MetricCard({ label, value, unit, description, tone = '', empty }) {
  return <div className={`metric-card ${tone}`}><h3>{label}</h3><div className="metric-value">{empty ? '—' : value}<span>{!empty && value !== 'N/A' ? unit : ''}</span></div><p>{description}</p></div>
}
