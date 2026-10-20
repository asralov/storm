export default function Header({ health, page = 'dashboard' }) {
  const labels = { online: 'Backend connected', offline: 'Backend unavailable', checking: 'Checking backend' }
  return <header className="page-header"><div><div className="eyebrow">WORKSPACE <span>/</span> {page === 'history' ? 'HISTORY' : 'OVERVIEW'}</div><h1>{page === 'history' ? 'Test History' : 'Dashboard'}</h1><p>{page === 'history' ? 'Revisit saved performance reports on this machine.' : 'Understand how your API performs under pressure.'}</p></div><div className="connection-block"><span className={`connection ${health}`} role="status"><span className="dot" />{labels[health]}</span><span className="header-caption">Local HTTP testing</span></div></header>
}
