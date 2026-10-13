import { useEffect, useState } from 'react'
import './App.css'
import TestForm from './components/TestForm'

function App() {
  const [status, setStatus] = useState('Connecting...')

  useEffect(() => {
    fetch('/api/health')
      .then((response) => {
        if (!response.ok) {
          throw new Error('Backend unavailable')
        }
        return response.json()
      })
      .then((data) => setStatus(data.status))
      .catch(() => setStatus('disconnected'))
  }, [])

  return (
    <main>
      <h1>Storm</h1>
      <p>Local API Performance Testing Platform</p>
      <TestForm />

    </main>
  )
}

export default App