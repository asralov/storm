import Icon from '../ui/Icon'
export default function Sidebar({ version, page = 'dashboard' }) {
  return (
    <aside className="sidebar">
      <a href="#dashboard" className="brand" aria-label="Storm dashboard"><span className="brand-mark"><Icon name="storm" size={25} /></span>storm<span className="brand-period">.</span></a>
      <div className="sidebar-section">WORKSPACE</div>
      <nav aria-label="Main navigation">
        <a className={`nav-item ${page === 'dashboard' ? 'active' : ''}`} href="#dashboard" aria-current={page === 'dashboard' ? 'page' : undefined}><Icon name="dashboard" />Dashboard<Icon name="arrow" size={15} /></a>
        <a className={`nav-item ${page === 'history' ? 'active' : ''}`} href="#history" aria-current={page === 'history' ? 'page' : undefined}><Icon name="history" />Test History</a>
        <button className="nav-item" disabled title="Settings — coming soon"><Icon name="settings" />Settings<span className="soon">Soon</span></button>
      </nav>
      <div className="sidebar-bottom">
        <div className="local-note"><Icon name="shield" /><strong>Local by design</strong><p>Your tests run on your machine. No cloud account required.</p></div>
        <div className="version"><span className="dot" />LOCAL WORKSPACE<span>{version ? `v${version}` : 'Version unavailable'}</span></div>
      </div>
    </aside>
  )
}
