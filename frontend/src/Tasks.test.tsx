import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import type { Task } from './api/tasks'

interface FetchCall {
  url: string
  init?: RequestInit
}

interface RouteHandler {
  match: (url: string, init?: RequestInit) => boolean
  respond: (url: string, init?: RequestInit) => { status: number; body: unknown }
}

const HEALTH_OK: RouteHandler = {
  match: (url) => url === '/healthz',
  respond: () => ({ status: 200, body: { status: 'ok' } }),
}

function jsonResponse(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  }
}

function createFetchMock(handlers: RouteHandler[]) {
  const calls: FetchCall[] = []
  const fn = vi.fn((url: string, init?: RequestInit) => {
    calls.push({ url, init })
    const handler = handlers.find((h) => h.match(url, init))
    if (!handler) {
      return Promise.reject(new Error(`no handler registered for ${String(init?.method ?? 'GET')} ${url}`))
    }
    const { status, body } = handler.respond(url, init)
    return Promise.resolve(jsonResponse(status, body))
  })
  return { fn, calls }
}

const task1: Task = {
  id: 't1',
  title: 'Comprar leite',
  description: '',
  status: 'todo',
  archived: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function listHandler(archived: boolean, tasks: Task[]): RouteHandler {
  return {
    match: (url, init) =>
      (!init?.method || init.method === 'GET') &&
      url === `/api/v1/tasks?archived=${archived}`,
    respond: () => ({ status: 200, body: { tasks } }),
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Tasks feature', () => {
  it('renders the tasks returned by the API', async () => {
    const { fn } = createFetchMock([HEALTH_OK, listHandler(false, [task1])])
    vi.stubGlobal('fetch', fn)

    render(<App />)

    expect(await screen.findByText('Comprar leite')).toBeInTheDocument()
  })

  it('shows the empty state when the list has no tasks', async () => {
    const { fn } = createFetchMock([HEALTH_OK, listHandler(false, [])])
    vi.stubGlobal('fetch', fn)

    render(<App />)

    expect(await screen.findByText('Nenhuma tarefa ainda')).toBeInTheDocument()
  })

  it('creates a task: sends the POST body and shows the new row', async () => {
    const created: Task = {
      id: 't2',
      title: 'Nova tarefa',
      description: 'Descrição da tarefa',
      status: 'todo',
      archived: false,
      created_at: '2026-01-02T00:00:00Z',
      updated_at: '2026-01-02T00:00:00Z',
    }
    const createHandler: RouteHandler = {
      match: (url, init) => url === '/api/v1/tasks' && init?.method === 'POST',
      respond: () => ({ status: 201, body: created }),
    }
    const { fn, calls } = createFetchMock([HEALTH_OK, listHandler(false, []), createHandler])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText('Nenhuma tarefa ainda')

    fireEvent.change(screen.getByLabelText('Título'), { target: { value: 'Nova tarefa' } })
    fireEvent.change(screen.getByLabelText('Descrição'), {
      target: { value: 'Descrição da tarefa' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Adicionar' }))

    expect(await screen.findByText('Nova tarefa')).toBeInTheDocument()

    const postCall = calls.find((c) => c.url === '/api/v1/tasks' && c.init?.method === 'POST')
    expect(postCall).toBeDefined()
    expect(JSON.parse(postCall!.init!.body as string)).toEqual({
      title: 'Nova tarefa',
      description: 'Descrição da tarefa',
    })
  })

  it('shows the field message on 422 and keeps the typed text', async () => {
    const createHandler: RouteHandler = {
      match: (url, init) => url === '/api/v1/tasks' && init?.method === 'POST',
      respond: () => ({
        status: 422,
        body: {
          error: {
            code: 'VALIDATION_ERROR',
            message: 'Dados inválidos',
            fields: { title: 'Título é obrigatório' },
          },
        },
      }),
    }
    const { fn } = createFetchMock([HEALTH_OK, listHandler(false, []), createHandler])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText('Nenhuma tarefa ainda')

    fireEvent.change(screen.getByLabelText('Título'), { target: { value: '   ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Adicionar' }))

    expect(await screen.findByText('Título é obrigatório')).toBeInTheDocument()
    expect(screen.getByLabelText('Título')).toHaveValue('   ')
  })

  it('changing the status select issues a PATCH and reflects the new status', async () => {
    const updated: Task = { ...task1, status: 'done' }
    const patchHandler: RouteHandler = {
      match: (url, init) => url === `/api/v1/tasks/${task1.id}` && init?.method === 'PATCH',
      respond: () => ({ status: 200, body: updated }),
    }
    const { fn, calls } = createFetchMock([HEALTH_OK, listHandler(false, [task1]), patchHandler])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    const select = await screen.findByLabelText(`Status de ${task1.title}`)

    fireEvent.change(select, { target: { value: 'done' } })

    await waitFor(() => expect(select).toHaveValue('done'))

    const patchCall = calls.find(
      (c) => c.url === `/api/v1/tasks/${task1.id}` && c.init?.method === 'PATCH',
    )
    expect(patchCall).toBeDefined()
    expect(JSON.parse(patchCall!.init!.body as string)).toEqual({ status: 'done' })
  })

  it('archiving a task removes it from the default list', async () => {
    const archiveHandler: RouteHandler = {
      match: (url, init) =>
        url === `/api/v1/tasks/${task1.id}/archive` && init?.method === 'POST',
      respond: () => ({ status: 200, body: { ...task1, archived: true } }),
    }
    const { fn } = createFetchMock([HEALTH_OK, listHandler(false, [task1]), archiveHandler])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText(task1.title)

    fireEvent.click(screen.getByRole('button', { name: 'Arquivar' }))

    await waitFor(() => expect(screen.queryByText(task1.title)).not.toBeInTheDocument())
  })

  it('toggling "mostrar arquivadas" lists archived tasks and offers "Restaurar"', async () => {
    const archivedTask: Task = { ...task1, archived: true }
    const { fn } = createFetchMock([
      HEALTH_OK,
      listHandler(false, []),
      listHandler(true, [archivedTask]),
    ])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText('Nenhuma tarefa ainda')

    fireEvent.click(screen.getByLabelText('Mostrar arquivadas'))

    expect(await screen.findByText(archivedTask.title)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Restaurar' })).toBeInTheDocument()
  })

  it('shows an error message on network failure without crashing', async () => {
    const fn = vi.fn((url: string) => {
      if (url === '/healthz') {
        return Promise.resolve(jsonResponse(200, { status: 'ok' }))
      }
      if (url === '/api/v1/tasks?archived=false') {
        return Promise.reject(new Error('network error'))
      }
      return Promise.reject(new Error(`no handler for ${url}`))
    })
    vi.stubGlobal('fetch', fn)

    render(<App />)

    expect(
      await screen.findByText('Não foi possível carregar as tarefas. Tente novamente.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'SSDLC Example' })).toBeInTheDocument()
  })
})
