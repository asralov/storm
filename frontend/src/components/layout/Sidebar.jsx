import Icon from '../ui/Icon'
export default function Sidebar({ version }) {
  return (
    <aside className="sidebar">
      <a href="#dashboard" className="brand" aria-label="Storm dashboard"><span className="brand-mark"><Icon name="storm" size={25} /></span>storm<span className="brand-period">.</span></a>
      <div className="sidebar-section">WORKSPACE</div>
      <nav aria-label="Main navigation">
        <a className="nav-item active" href="#dashboard" aria-current="page"><Icon name="dashboard" />Dashboard<Icon name="arrow" size={15} /></a>
        <button className="nav-item" disabled title="Test History — coming soon"><Icon name="history" />Test History<span className="soon">Soon</span></button>
        <button className="nav-item" disabled title="Settings — coming soon"><Icon name="settings" />Settings<span className="soon">Soon</span></button>
      </nav>
      <div className="sidebar-bottom">
        <div className="local-note"><Icon name="shield" /><strong>Local by design</strong><p>Your tests run on your machine. No cloud account required.</p></div>
        <div className="version"><span className="dot" />LOCAL WORKSPACE<span>{version ? `v${version}` : 'Version unavailable'}</span></div>
      </div>
    </aside>
  )
}
