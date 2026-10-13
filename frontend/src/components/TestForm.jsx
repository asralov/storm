import {useState} from 'react'
import {createTest} from '../services/testService'

export default function TestForm() {
    const [url, setUrl] = useState('http://127.0.0.1:8081')
    const [workers, setWorkers] = useState(2)
    const [duration, setDuration] = useState(5)
    const [authorized, setAuthorized] = useState(false)

    const [loading, setLoading] = useState(false)
    const [result, setResult] = useState(null)
    const [error, setError] = useState('')

    async function handleSubmit(event) {
        event.preventDefault()

        setLoading(true)
        setError('')
        setResult(null)

        try {
            const config = {
                url,
                method: 'GET',
                workers: Number(workers),
                duration_seconds: Number(duration),
                authorized
            }

            const response = await createTest(config)
            setResult(response)
        } catch (err) {
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    return (
        <form onSubmit={handleSubmit}>
            <h2>Configure Load Test</h2>

            <label htmlFor="target-url">Target URL</label>
            <input
                id="target-url"
                type='url'
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                required
            />

            <label htmlFor="workers">Number of Workers</label>
            <input
                id="workers"
                type='number'
                value={workers}
                min="1"
                max="10"
                onChange={(e) => setWorkers(e.target.value)}
                required
            />

            <label htmlFor="duration">Test Duration (seconds)</label>
            <input
                id="duration"
                type='number'
                value={duration}
                min="1"
                max="30"
                onChange={(e) => setDuration(e.target.value)}
                required
            />

            <label htmlFor="authorized">
                <input
                    id="authorized"
                    type='checkbox'
                    checked={authorized}
                    onChange={(e) => setAuthorized(e.target.checked)}
                />
                I own this API or have permission to load test it
            </label>

            <button type="submit" disabled={loading || !authorized}>
                {loading ? 'Submitting...' : 'Validate Test Configuration'}
            </button>

            {error && <p className="error">{error}</p>}
            
            {result && (
            <section className="results">
                <h2>Test Results</h2>

                <p>Total Requests: {result.requests}</p>
                <p>Successful: {result.successes}</p>
                <p>Failed: {result.failures}</p>

                <p>
                Throughput: {result.throughput.toFixed(2)} req/s
                </p>

                <p>
                Average Latency: {result.avg_latency_ms.toFixed(2)} ms
                </p>

                <p>
                Elapsed: {result.elapsed_seconds.toFixed(2)} seconds
                </p>
            </section>
            )}

        </form>
    )
}
