import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import type { Task } from './api/tasks'
import type { TimeEntry } from './api/timer'

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

function listHandler(archived: boolean, tasks: Task[]): RouteHandler {
  return {
    match: (url, init) => (!init?.method || init.method === 'GET') && url === `/api/v1/tasks?archived=${archived}`,
    respond: () => ({ status: 200, body: { tasks } }),
  }
}

function timerHandler(body: { entry: TimeEntry; task: Task } | null): RouteHandler {
  return {
    match: (url, init) => (!init?.method || init.method === 'GET') && url === '/api/v1/timer',
    respond: () => ({ status: 200, body }),
  }
}

function entriesHandler(taskId: string, entries: TimeEntry[]): RouteHandler {
  return {
    match: (url, init) => (!init?.method || init.method === 'GET') && url === `/api/v1/tasks/${taskId}/time-entries`,
    respond: () => ({ status: 200, body: { entries } }),
  }
}

const task1: Task = {
  id: 't1',
  title: 'Comprar leite',
  description: '',
  status: 'todo',
  archived: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  total_seconds: 0,
}

const activeEntry: TimeEntry = {
  id: 'e1',
  task_id: 't1',
  started_at: '2026-01-01T00:00:00.000Z',
  ended_at: null,
  duration_seconds: null,
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('Timer bar', () => {
  it('renders HH:MM:SS and advances by one second after 1s (AC9)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.setSystemTime(new Date('2026-01-01T00:00:00.000Z'))
    const { fn } = createFetchMock([
      HEALTH_OK,
      listHandler(false, [task1]),
      timerHandler({ entry: activeEntry, task: task1 }),
    ])
    vi.stubGlobal('fetch', fn)

    render(<App />)

    expect(await screen.findByText('00:00:00')).toBeInTheDocument()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000)
    })
    expect(screen.getByText('00:00:05')).toBeInTheDocument()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000)
    })
    expect(screen.getByText('00:00:06')).toBeInTheDocument()
  })

  it('restores a running timer on mount from GET /api/v1/timer and shows the task title (AC10)', async () => {
    const { fn } = createFetchMock([
      HEALTH_OK,
      listHandler(false, [task1]),
      timerHandler({ entry: activeEntry, task: task1 }),
    ])
    vi.stubGlobal('fetch', fn)

    render(<App />)

    expect(await screen.findByText(task1.title, { selector: 'output span' })).toBeInTheDocument()
  })

  it('does not show the bar when there is no active timer', async () => {
    const { fn } = createFetchMock([HEALTH_OK, listHandler(false, [task1]), timerHandler(null)])
    vi.stubGlobal('fetch', fn)

    render(<App />)

    await screen.findByText(task1.title)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('"Iniciar" calls the start endpoint and the bar appears', async () => {
    let timerState: { entry: TimeEntry; task: Task } | null = null
    const startHandler: RouteHandler = {
      match: (url, init) => url === `/api/v1/tasks/${task1.id}/timer/start` && init?.method === 'POST',
      respond: () => {
        timerState = { entry: activeEntry, task: task1 }
        return { status: 201, body: activeEntry }
      },
    }
    const timerRoute: RouteHandler = {
      match: (url, init) => (!init?.method || init.method === 'GET') && url === '/api/v1/timer',
      respond: () => ({ status: 200, body: timerState }),
    }
    const { fn, calls } = createFetchMock([HEALTH_OK, listHandler(false, [task1]), timerRoute, startHandler])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText(task1.title)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Iniciar' }))

    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument())
    const startCall = calls.find((c) => c.url === `/api/v1/tasks/${task1.id}/timer/start`)
    expect(startCall).toBeDefined()
  })

  it('"Parar" calls stop and the bar disappears', async () => {
    const stopHandler: RouteHandler = {
      match: (url, init) => url === '/api/v1/timer/stop' && init?.method === 'POST',
      respond: () => ({ status: 200, body: { ...activeEntry, ended_at: '2026-01-01T00:00:05.000Z', duration_seconds: 5 } }),
    }
    const { fn, calls } = createFetchMock([
      HEALTH_OK,
      listHandler(false, [task1]),
      timerHandler({ entry: activeEntry, task: task1 }),
      stopHandler,
    ])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Parar' }))

    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument())
    const stopCall = calls.find((c) => c.url === '/api/v1/timer/stop')
    expect(stopCall).toBeDefined()
  })

  it('"Iniciar" is absent for a done task', async () => {
    const doneTask: Task = { ...task1, id: 't-done', title: 'Tarefa concluída', status: 'done' }
    const { fn } = createFetchMock([HEALTH_OK, listHandler(false, [doneTask]), timerHandler(null)])
    vi.stubGlobal('fetch', fn)

    render(<App />)

    await screen.findByText(doneTask.title)
    expect(screen.queryByRole('button', { name: 'Iniciar' })).not.toBeInTheDocument()
  })

  it('"Iniciar" is absent for an archived task', async () => {
    const archivedTask: Task = { ...task1, id: 't-arch', title: 'Tarefa arquivada', archived: true }
    const { fn } = createFetchMock([
      HEALTH_OK,
      listHandler(false, []),
      listHandler(true, [archivedTask]),
      timerHandler(null),
    ])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText('Nenhuma tarefa ainda')

    fireEvent.click(screen.getByLabelText('Mostrar arquivadas'))

    await screen.findByText(archivedTask.title)
    expect(screen.queryByRole('button', { name: 'Iniciar' })).not.toBeInTheDocument()
  })

  it('shows the task total as HH:MM derived from total_seconds', async () => {
    const taskWithTotal: Task = { ...task1, total_seconds: 5400 }
    const { fn } = createFetchMock([HEALTH_OK, listHandler(false, [taskWithTotal]), timerHandler(null)])
    vi.stubGlobal('fetch', fn)

    render(<App />)

    expect(await screen.findByText('Total: 01:30')).toBeInTheDocument()
  })
})

describe('Time entries', () => {
  const finishedEntry: TimeEntry = {
    id: 'e2',
    task_id: task1.id,
    started_at: '2026-01-01T10:00:00.000Z',
    ended_at: '2026-01-01T10:00:30.000Z',
    duration_seconds: 30,
  }

  it('lists entries with duration; deleting asks for confirmation before calling DELETE', async () => {
    const deleteHandler: RouteHandler = {
      match: (url, init) => url === `/api/v1/time-entries/${finishedEntry.id}` && init?.method === 'DELETE',
      respond: () => ({ status: 204, body: undefined }),
    }
    const { fn, calls } = createFetchMock([
      HEALTH_OK,
      listHandler(false, [task1]),
      timerHandler(null),
      entriesHandler(task1.id, [finishedEntry]),
      deleteHandler,
    ])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText(task1.title)

    fireEvent.click(screen.getByRole('button', { name: 'Ver apontamentos' }))
    expect(await screen.findByText('00:00:30')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Excluir' }))
    expect(screen.getByText('Excluir este apontamento?')).toBeInTheDocument()
    expect(calls.some((c) => c.init?.method === 'DELETE')).toBe(false)

    fireEvent.click(screen.getByRole('button', { name: 'Confirmar exclusão' }))

    await waitFor(() => expect(calls.some((c) => c.init?.method === 'DELETE')).toBe(true))
    await waitFor(() => expect(screen.queryByText('00:00:30')).not.toBeInTheDocument())
  })

  it('editing an entry that returns 422 shows the message and keeps the inputs open', async () => {
    const patchHandler: RouteHandler = {
      match: (url, init) => url === `/api/v1/time-entries/${finishedEntry.id}` && init?.method === 'PATCH',
      respond: () => ({
        status: 422,
        body: { error: { code: 'VALIDATION_ERROR', message: 'Fim deve ser depois do início' } },
      }),
    }
    const { fn } = createFetchMock([
      HEALTH_OK,
      listHandler(false, [task1]),
      timerHandler(null),
      entriesHandler(task1.id, [finishedEntry]),
      patchHandler,
    ])
    vi.stubGlobal('fetch', fn)

    render(<App />)
    await screen.findByText(task1.title)
    fireEvent.click(screen.getByRole('button', { name: 'Ver apontamentos' }))
    await screen.findByText('00:00:30')

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }))
    expect(screen.getByLabelText('Início')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Salvar' }))

    expect(await screen.findByText('Fim deve ser depois do início')).toBeInTheDocument()
    expect(screen.getByLabelText('Início')).toBeInTheDocument()
  })
})

describe('useTimer cleanup', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
  })

  it('clears the interval on unmount', async () => {
    const clearSpy = vi.spyOn(globalThis, 'clearInterval')
    const { fn } = createFetchMock([
      HEALTH_OK,
      listHandler(false, [task1]),
      timerHandler({ entry: activeEntry, task: task1 }),
    ])
    vi.stubGlobal('fetch', fn)

    const { unmount } = render(<App />)
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument())

    unmount()

    expect(clearSpy).toHaveBeenCalled()
  })
})
