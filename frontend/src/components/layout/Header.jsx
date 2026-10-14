export default function Header({ health }) {
  const labels = { online: 'Backend connected', offline: 'Backend unavailable', checking: 'Checking backend' }
  return <header className="page-header"><div><div className="eyebrow">WORKSPACE <span>/</span> OVERVIEW</div><h1>Dashboard</h1><p>Understand how your API performs under pressure.</p></div><div className="connection-block"><span className={`connection ${health}`} role="status"><span className="dot" />{labels[health]}</span><span className="header-caption">Local HTTP testing</span></div></header>
}
