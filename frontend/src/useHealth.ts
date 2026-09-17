import { useEffect, useState } from 'react'

export type HealthStatus = 'loading' | 'online' | 'offline'

interface HealthResponse {
  status?: string
}

export function useHealth(): HealthStatus {
  const [status, setStatus] = useState<HealthStatus>('loading')

  useEffect(() => {
    let cancelled = false

    async function checkHealth() {
      try {
        const response = await fetch('/healthz')
        if (!response.ok) {
          if (!cancelled) setStatus('offline')
          return
        }
        const data = (await response.json()) as HealthResponse
        if (!cancelled) setStatus(data.status === 'ok' ? 'online' : 'offline')
      } catch {
        if (!cancelled) setStatus('offline')
      }
    }

    checkHealth()

    return () => {
      cancelled = true
    }
  }, [])

  return status
}
