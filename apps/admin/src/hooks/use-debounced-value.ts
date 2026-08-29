import { useEffect, useState } from 'react'

export function useDebouncedValue<T>(value: T, delayMilliseconds = 200) {
  const [debouncedValue, setDebouncedValue] = useState(value)

  useEffect(() => {
    const timeout = window.setTimeout(() => setDebouncedValue(value), delayMilliseconds)
    return () => window.clearTimeout(timeout)
  }, [delayMilliseconds, value])

  return debouncedValue
}
