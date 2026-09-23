export type TaskStatus = 'todo' | 'in_progress' | 'done'



export interface Task {
  id: string
  title: string
  description: string
  status: TaskStatus
  archived: boolean
  created_at: string
  updated_at: string
  total_seconds: number
}

interface ApiErrorBody {
  error: {
    code: string
    message: string
    fields?: Record<string, string>
  }
}

export class TaskApiError extends Error {
  status: number
  code: string
  fields?: Record<string, string>

  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'TaskApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }
}

export async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let body: ApiErrorBody | undefined
    try {
      body = (await response.json()) as ApiErrorBody
    } catch {
      body = undefined
    }
    const code = body?.error?.code ?? 'UNKNOWN_ERROR'
    const message = body?.error?.message ?? 'Erro inesperado'
    throw new TaskApiError(response.status, code, message, body?.error?.fields)
  }
  return (await response.json()) as T
}

export async function listTasks(archived: boolean): Promise<Task[]> {
  const response = await fetch(`/api/v1/tasks?archived=${archived}`)
  const data = await handleResponse<{ tasks: Task[] }>(response)
  if (!Array.isArray(data.tasks)) {
    throw new TypeError('Resposta inválida do servidor')
  }
  return data.tasks
}

export async function createTask(title: string, description: string): Promise<Task> {
  const response = await fetch('/api/v1/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, description }),
  })
  return handleResponse<Task>(response)
}

export async function updateTask(
  id: string,
  patch: Partial<Pick<Task, 'title' | 'description' | 'status'>>,
): Promise<Task> {
  const response = await fetch(`/api/v1/tasks/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  return handleResponse<Task>(response)
}

export async function archiveTask(id: string): Promise<Task> {
  const response = await fetch(`/api/v1/tasks/${id}/archive`, { method: 'POST' })
  return handleResponse<Task>(response)
}

export async function restoreTask(id: string): Promise<Task> {
  const response = await fetch(`/api/v1/tasks/${id}/restore`, { method: 'POST' })
  return handleResponse<Task>(response)
}
