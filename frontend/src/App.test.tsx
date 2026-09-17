import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'

function mockFetchResolved(body: unknown, status = 200) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('App', () => {
  it('shows the heading "SSDLC Example"', () => {
    vi.stubGlobal('fetch', mockFetchResolved({ status: 'ok' }))

    render(<App />)

    expect(screen.getByRole('heading', { name: 'SSDLC Example' })).toBeInTheDocument()
  })

  it('shows "API online" when /healthz responds 200 with status ok', async () => {
    vi.stubGlobal('fetch', mockFetchResolved({ status: 'ok' }))

    render(<App />)

    expect(await screen.findByText('API online')).toBeInTheDocument()
  })

  it('shows "API indisponível" and not "API online" when fetch rejects', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockRejectedValue(new Error('network error')),
    )

    render(<App />)

    expect(await screen.findByText('API indisponível')).toBeInTheDocument()
    expect(screen.queryByText('API online')).not.toBeInTheDocument()
  })

  it('shows "API indisponível" and not "API online" when /healthz responds 500', async () => {
    vi.stubGlobal('fetch', mockFetchResolved({}, 500))

    render(<App />)

    expect(await screen.findByText('API indisponível')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByText('API online')).not.toBeInTheDocument(),
    )
  })
})
