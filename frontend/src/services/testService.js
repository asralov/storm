async function request(path, { signal, ...options } = {}) {
  const timeout = AbortSignal.timeout(5000)
  let response
  try {
    response = await fetch(path, { ...options, signal: signal ? AbortSignal.any([signal, timeout]) : timeout })
  } catch (error) {
    if (signal?.aborted) throw error
    throw new Error('Cannot reach Storm. Check that the backend is running and try again.', { cause: error })
  }
  const data = await response.json().catch(() => {
    throw new Error('Storm returned an unreadable response. Check the backend and retry.')
  })
  if (!response.ok) {
    const error = new Error(data.error || 'Request failed')
    error.status = response.status
    throw error
  }
  return data
}

export const createTest = (config, signal) => request('/api/tests', {
  method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(config), signal,
})
export const getTest = (id, signal) => request(`/api/tests/${encodeURIComponent(id)}`, { signal })
export const getActiveTest = signal => request('/api/tests/active', { signal })
export const listHistory = (offset = 0, signal) => request(`/api/history?limit=20&offset=${offset}`, { signal })
export const getHistory = (id, signal) => request(`/api/history/${encodeURIComponent(id)}`, { signal })
export const deleteHistory = (id, signal) => request(`/api/history/${encodeURIComponent(id)}`, {
  method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: '{}', signal,
})
export const stopTest = (id, signal) => request(`/api/tests/${encodeURIComponent(id)}/stop`, {
  method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}', signal,
})
export async function getHealth(signal) {
  const data = await request('/api/health', { signal })
  if (data.status !== 'ok') throw new Error('Backend unavailable')
  return data
}
