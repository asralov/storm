export function stagePlan(start, increment, maximum, duration) {
  if (![start, increment, maximum, duration].every(Number.isInteger)) return { rates: [], error: 'Enter whole numbers for the stage settings.' }
  if (start < 1 || maximum > 100 || maximum < start || increment < 1 || increment > 100) return { rates: [], error: 'Rates must be 1–100 req/s, with maximum at least start and a positive step.' }
  if (duration < 1 || duration > 30) return { rates: [], error: 'Each stage must last 1–30 seconds.' }
  const count = Math.floor((maximum - start) / increment) + 1
  if (count > 10 || count * duration > 120) return { rates: [], error: 'Use at most 10 stages and 120 seconds of total configured load.' }
  return { rates: Array.from({ length: count }, (_, i) => start + i * increment), totalSeconds: count * duration, error: '' }
}
