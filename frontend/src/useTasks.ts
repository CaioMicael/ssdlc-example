import { useCallback, useEffect, useState } from 'react'
import { archiveTask, createTask, listTasks, restoreTask, updateTask } from './api/tasks'
import type { Task, TaskStatus } from './api/tasks'

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showArchived, setShowArchived] = useState(false)

  useEffect(() => {
    let cancelled = false

    async function load() {
      try {
        const result = await listTasks(showArchived)
        if (!cancelled) {
          setTasks(result)
          setError(null)
        }
      } catch {
        if (!cancelled) setError('Não foi possível carregar as tarefas. Tente novamente.')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    load()

    return () => {
      cancelled = true
    }
  }, [showArchived])

  const create = useCallback(
    async (title: string, description: string) => {
      const task = await createTask(title, description)
      setTasks((prev) => (showArchived ? prev : [task, ...prev]))
      return task
    },
    [showArchived],
  )

  const changeStatus = useCallback(async (id: string, status: TaskStatus) => {
    try {
      const updated = await updateTask(id, { status })
      setTasks((prev) => prev.map((task) => (task.id === id ? updated : task)))
    } catch {
      setError('Não foi possível atualizar a tarefa. Tente novamente.')
    }
  }, [])

  const archive = useCallback(async (id: string) => {
    try {
      await archiveTask(id)
      setTasks((prev) => prev.filter((task) => task.id !== id))
    } catch {
      setError('Não foi possível arquivar a tarefa. Tente novamente.')
    }
  }, [])

  const restore = useCallback(async (id: string) => {
    try {
      await restoreTask(id)
      setTasks((prev) => prev.filter((task) => task.id !== id))
    } catch {
      setError('Não foi possível restaurar a tarefa. Tente novamente.')
    }
  }, [])

  const toggleArchived = useCallback(() => {
    setShowArchived((prev) => !prev)
  }, [])

  return {
    tasks,
    loading,
    error,
    showArchived,
    create,
    changeStatus,
    archive,
    restore,
    toggleArchived,
  }
}
