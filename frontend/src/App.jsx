import { useEffect, useState } from 'react'
import './App.css'
import Sidebar from './components/layout/Sidebar'
import Header from './components/layout/Header'
import TestForm from './components/TestForm'
import { getHealth } from './services/testService'

export default function App() {
  const [health, setHealth] = useState({ state: 'checking' })
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
      <a className="skip-link" href="#dashboard">Skip to dashboard</a>
      <Sidebar version={health.version} />
      <main id="dashboard" className="main-content">
        <Header health={health.state} />
        <TestForm />
        <footer className="page-footer"><span>Built for your local environment.</span><span>STORM / HTTP PERFORMANCE</span></footer>
      </main>
    </div>
  )
}
