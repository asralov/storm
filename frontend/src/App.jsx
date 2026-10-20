import { useEffect, useState } from 'react'
import './App.css'
import Sidebar from './components/layout/Sidebar'
import Header from './components/layout/Header'
import TestForm from './components/TestForm'
import { getHealth } from './services/testService'
import HistoryPage from './components/history/HistoryPage'

export default function App() {
  const [health, setHealth] = useState({ state: 'checking' })
  const [route, setRoute] = useState(() => window.location.hash.slice(1))
  const history = route === 'history' || route.startsWith('history/')
  useEffect(() => { const update = () => setRoute(window.location.hash.slice(1)); window.addEventListener('hashchange', update); return () => window.removeEventListener('hashchange', update) }, [])
  useEffect(() => {
    const controller = new AbortController()
    let timer
    async function check() {
      try {
        const data = await getHealth(controller.signal)
        if (!controller.signal.aborted) setHealth({ state: 'online', version: data.version })
      } catch {
        if (!controller.signal.aborted) setHealth({ state: 'offline' })
      }
      if (!controller.signal.aborted) timer = setTimeout(check, 15000)
    }
    check()
    return () => { controller.abort(); clearTimeout(timer) }
  }, [])
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content" onClick={e => { e.preventDefault(); document.getElementById('main-content').focus() }}>Skip to main content</a>
      <Sidebar version={health.version} page={history ? 'history' : 'dashboard'} />
      <main id="main-content" className="main-content" tabIndex={-1}>
        <Header health={health.state} page={history ? 'history' : 'dashboard'} />
        <div hidden={history}><TestForm visible={!history} /></div>
        {history && <HistoryPage id={route.startsWith('history/') ? route.slice(8) : undefined} />}
        <footer className="page-footer"><span>Built for your local environment.</span><span>STORM / HTTP PERFORMANCE</span></footer>
      </main>
    </div>
  )
}
