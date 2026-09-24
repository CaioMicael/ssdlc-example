import { handleResponse } from './tasks'
import type { Task } from './tasks'

export interface TimeEntry {
  id: string
  task_id: string
  started_at: string
  ended_at: string | null
  duration_seconds: number | null
}

export interface ActiveTimer {
  entry: TimeEntry
  task: Task
}

export async function startTimer(taskId: string): Promise<TimeEntry> {
  const response = await fetch(`/api/v1/tasks/${taskId}/timer/start`, { method: 'POST' })
  return handleResponse<TimeEntry>(response)
}

export async function stopTimer(): Promise<TimeEntry> {
  const response = await fetch('/api/v1/timer/stop', { method: 'POST' })
  return handleResponse<TimeEntry>(response)
}

export async function getActiveTimer(): Promise<ActiveTimer | null> {
  const response = await fetch('/api/v1/timer')
  const data = await handleResponse<ActiveTimer | null>(response)
  return data ?? null
}

export async function listEntries(taskId: string): Promise<TimeEntry[]> {
  const response = await fetch(`/api/v1/tasks/${taskId}/time-entries`)
  const data = await handleResponse<{ entries: TimeEntry[] }>(response)
  return data.entries
}

export async function updateEntry(
  id: string,
  patch: { started_at?: string; ended_at?: string },
): Promise<TimeEntry> {
  const response = await fetch(`/api/v1/time-entries/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  return handleResponse<TimeEntry>(response)
}

export async function deleteEntry(id: string): Promise<void> {
  const response = await fetch(`/api/v1/time-entries/${id}`, { method: 'DELETE' })
  if (response.ok) return
  await handleResponse<void>(response)
}
