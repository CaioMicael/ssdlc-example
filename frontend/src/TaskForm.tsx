import { useState } from 'react'
import type { FormEvent } from 'react'
import { TaskApiError } from './api/tasks'
import type { Task } from './api/tasks'

interface TaskFormProps {
  onCreate: (title: string, description: string) => Promise<Task>
}

export function TaskForm({ onCreate }: Readonly<TaskFormProps>) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [generalError, setGeneralError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFieldErrors({})
    setGeneralError(null)
    setSubmitting(true)
    try {
      await onCreate(title, description)
      setTitle('')
      setDescription('')
    } catch (err) {
      if (err instanceof TaskApiError && err.fields) {
        setFieldErrors(err.fields)
      } else {
        setGeneralError('Não foi possível criar a tarefa. Tente novamente.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="card" onSubmit={handleSubmit}>
      <div>
        <label htmlFor="task-title">Título</label>
        <input id="task-title" value={title} onChange={(e) => setTitle(e.target.value)} />
        {fieldErrors.title && <p role="alert">{fieldErrors.title}</p>}
      </div>
      <div>
        <label htmlFor="task-description">Descrição</label>
        <textarea
          id="task-description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        {fieldErrors.description && <p role="alert">{fieldErrors.description}</p>}
      </div>
      {generalError && <p role="alert">{generalError}</p>}
      <button type="submit" disabled={submitting}>
        Adicionar
      </button>
    </form>
  )
}
