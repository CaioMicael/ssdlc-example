import { useEffect, useState } from 'react'
import { deleteEntry, listEntries, updateEntry } from './api/timer'
import type { TimeEntry } from './api/timer'
import { TaskApiError } from './api/tasks'
import { formatHms, fromLocalInputValue, toLocalInputValue } from './timeFormat'

interface TimeEntriesProps {
  taskId: string
}

export function TimeEntries({ taskId }: Readonly<TimeEntriesProps>) {
  const [entries, setEntries] = useState<TimeEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editStart, setEditStart] = useState('')
  const [editEnd, setEditEnd] = useState('')
  const [editError, setEditError] = useState<string | null>(null)
  const [confirmingId, setConfirmingId] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function load() {
      try {
        const result = await listEntries(taskId)
        if (!cancelled) {
          setEntries(result)
          setError(null)
        }
      } catch {
        if (!cancelled) setError('Não foi possível carregar os apontamentos. Tente novamente.')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    load()

    return () => {
      cancelled = true
    }
  }, [taskId])

  function startEdit(entry: TimeEntry) {
    setEditingId(entry.id)
    setEditStart(toLocalInputValue(entry.started_at))
    setEditEnd(entry.ended_at ? toLocalInputValue(entry.ended_at) : '')
    setEditError(null)
  }

  function cancelEdit() {
    setEditingId(null)
    setEditError(null)
  }

  async function saveEdit(id: string) {
    try {
      const patch: { started_at?: string; ended_at?: string } = {
        started_at: fromLocalInputValue(editStart),
      }
      if (editEnd) patch.ended_at = fromLocalInputValue(editEnd)
      const updated = await updateEntry(id, patch)
      setEntries((prev) => prev.map((entry) => (entry.id === id ? updated : entry)))
      setEditingId(null)
      setEditError(null)
    } catch (err) {
      setEditError(err instanceof TaskApiError ? err.message : 'Não foi possível salvar o apontamento.')
    }
  }

  async function confirmDelete(id: string) {
    try {
      await deleteEntry(id)
      setEntries((prev) => prev.filter((entry) => entry.id !== id))
      setConfirmingId(null)
    } catch {
      setError('Não foi possível excluir o apontamento. Tente novamente.')
      setConfirmingId(null)
    }
  }

  if (loading) return <p>Carregando apontamentos...</p>

  return (
    <div>
      {error && <p role="alert">{error}</p>}
      {entries.length === 0 ? (
        <p>Nenhum apontamento ainda</p>
      ) : (
        <ul>
          {entries.map((entry) => (
            <li key={entry.id}>
              {editingId === entry.id ? (
                <>
                  <label>
                    Início
                    <input
                      aria-label="Início"
                      type="datetime-local"
                      value={editStart}
                      onChange={(e) => setEditStart(e.target.value)}
                    />
                  </label>
                  <label>
                    Fim
                    <input
                      aria-label="Fim"
                      type="datetime-local"
                      value={editEnd}
                      onChange={(e) => setEditEnd(e.target.value)}
                    />
                  </label>
                  {editError && <p role="alert">{editError}</p>}
                  <button type="button" onClick={() => saveEdit(entry.id)}>
                    Salvar
                  </button>
                  <button type="button" onClick={cancelEdit}>
                    Cancelar
                  </button>
                </>
              ) : (
                <>
                  <span>{new Date(entry.started_at).toLocaleString('pt-BR')}</span>
                  {' - '}
                  <span>
                    {entry.ended_at ? new Date(entry.ended_at).toLocaleString('pt-BR') : 'em andamento'}
                  </span>
                  {' · '}
                  <span>
                    {entry.duration_seconds !== null ? formatHms(entry.duration_seconds) : '--:--:--'}
                  </span>{' '}
                  <button type="button" onClick={() => startEdit(entry)}>
                    Editar
                  </button>
                  {confirmingId === entry.id ? (
                    <>
                      <span>Excluir este apontamento?</span>
                      <button type="button" onClick={() => confirmDelete(entry.id)}>
                        Confirmar exclusão
                      </button>
                      <button type="button" onClick={() => setConfirmingId(null)}>
                        Cancelar
                      </button>
                    </>
                  ) : (
                    <button type="button" onClick={() => setConfirmingId(entry.id)}>
                      Excluir
                    </button>
                  )}
                </>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
