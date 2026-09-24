import { useCallback, useEffect, useReducer, useState } from 'react'
import { getActiveTimer, startTimer, stopTimer } from './api/timer'
import type { TimeEntry } from './api/timer'
import type { Task } from './api/tasks'

function elapsedSince(startedAt: string): number {
  const startMs = new Date(startedAt).getTime()
  return Math.max(0, Math.floor((Date.now() - startMs) / 1000))
}

export function useTimer() {
  const [activeEntry, setActiveEntry] = useState<TimeEntry | null>(null)
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  // Ticks once per second only to force a re-render; the displayed value below is
  // always recomputed from the server's started_at, never stored as its own state.
  const [, forceTick] = useReducer((tick: number) => tick + 1, 0)

  // AC10: restore the active timer from the server on mount/reload.
  useEffect(() => {
    let cancelled = false

    async function restore() {
      try {
        const active = await getActiveTimer()
        if (!cancelled && active?.entry && active.task) {
          setActiveEntry(active.entry)
          setActiveTask(active.task)
        }
      } catch {
        // sem cronômetro ativo ou falha ao consultar: mantém estado vazio
      }
    }

    restore()

    return () => {
      cancelled = true
    }
  }, [])

  // AC9: elapsed time is derived from the server's started_at; the interval only advances the display.
  useEffect(() => {
    if (!activeEntry) return undefined

    const interval = setInterval(() => {
      forceTick()
    }, 1000)

    return () => clearInterval(interval)
  }, [activeEntry])

  const elapsedSeconds = activeEntry ? elapsedSince(activeEntry.started_at) : 0

  const start = useCallback(async (taskId: string) => {
    await startTimer(taskId)
    const active = await getActiveTimer()
    if (active?.entry && active.task) {
      setActiveEntry(active.entry)
      setActiveTask(active.task)
    }
  }, [])

  const stop = useCallback(async () => {
    await stopTimer()
    setActiveEntry(null)
    setActiveTask(null)
  }, [])

  return { activeEntry, activeTask, elapsedSeconds, start, stop }
}
