import { useEffect, useRef, useState } from 'react'
import { createTest, getActiveTest, getTest, stopTest } from '../services/testService'
import { mergeRun } from './timeline'

const terminal = run => ['completed', 'canceled', 'failed'].includes(run.status) && run.persistence?.status !== 'saving'

export default function useTestRun() {
  const [run, setRun] = useState(null)
  const [recovering, setRecovering] = useState(true)
  const [starting, setStarting] = useState(false)
  const [stopping, setStopping] = useState(false)
  const [error, setError] = useState('')
  const mutation = useRef(false)
  const stopPending = useRef(false)
  const lifetime = useRef(null)
  const active = Boolean(run && !terminal(run))
  const id = run?.id

  useEffect(() => {
    const controller = new AbortController()
    lifetime.current = controller
    const historyDeleted = event => setRun(current => current?.id === event.detail.id ? { ...current, persistence: { status: 'deleted' } } : current)
    window.addEventListener('storm-history-deleted', historyDeleted)
    return () => { controller.abort(); window.removeEventListener('storm-history-deleted', historyDeleted) }
  }, [])

  useEffect(() => {
    if (!recovering) return
    const controller = new AbortController()
    let timer
    async function recover() {
      try {
        const data = await getActiveTest(controller.signal)
        if (!controller.signal.aborted) {
          setRun(previous => mergeRun(previous, data.run ? { ...data.run, chartRecovered: true } : null))
          setRecovering(false)
          setError('')
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(err.message)
          timer = setTimeout(recover, 1000)
        }
      }
    }
    recover()
    return () => { controller.abort(); clearTimeout(timer) }
  }, [recovering])

  useEffect(() => {
    if (!id || !active) return
    const controller = new AbortController()
    let timer
    async function poll() {
      let finished = false
      try {
        const data = await getTest(id, controller.signal)
        if (controller.signal.aborted) return
        setRun(previous => mergeRun(previous, data))
        setError('')
        finished = terminal(data)
      } catch (err) {
        if (controller.signal.aborted) return
        setError(err.message)
        if (err.status === 404) {
          // The backend may have restarted. Do not manufacture a final summary.
          setRun(current => ({ ...current, status: 'failed', persistence: undefined, error: 'Run unavailable. The backend may have restarted.' }))
          finished = true
        }
      }
      if (!finished && !controller.signal.aborted) timer = setTimeout(poll, 750)
    }
    poll()
    return () => { controller.abort(); clearTimeout(timer) }
  }, [id, active])

  async function submit(config) {
    if (mutation.current || active || recovering) return
    mutation.current = true
    setStarting(true)
    setError('')
    const signal = lifetime.current.signal
    try {
      const data = await createTest(config, signal)
      if (!signal.aborted) setRun(previous => mergeRun(previous, data))
    } catch (err) {
      if (signal.aborted) return
      setError(err.message)
      // A lost POST response or a 409 can mean a run already exists. Reconcile
      // before allowing another submission; never retry POST automatically.
      try {
        const data = await getActiveTest(signal)
        if (!signal.aborted && data.run) setRun(previous => mergeRun(previous, { ...data.run, chartRecovered: true }))
      } catch {
        if (!signal.aborted) {
          // Re-enter read-only recovery; never automatically resubmit a test.
          setError(`${err.message} Checking whether the run started before enabling another test.`)
          setRecovering(true)
        }
      }
    } finally {
      mutation.current = false
      if (!signal.aborted) setStarting(false)
    }
  }

  async function stop() {
    if (!active || stopPending.current || run.cancellation_requested || run.persistence?.status === 'saving') return
    stopPending.current = true
    setStopping(true)
    const signal = lifetime.current.signal
    try {
      await stopTest(id, signal)
      if (!signal.aborted) setRun(current => current?.id === id ? { ...current, cancellation_requested: true } : current)
    } catch (err) {
      if (!signal.aborted) setError(err.message)
    } finally {
      stopPending.current = false
      if (!signal.aborted) setStopping(false)
    }
  }

  return { run, active, recovering, starting, stopping, busy: active || recovering || starting, error, submit, stop }
}
