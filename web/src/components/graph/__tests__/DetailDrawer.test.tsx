import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import type { GraphNode } from '../../../lib/types'

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const { DetailDrawer } = await import('../DetailDrawer')

function renderDrawer(node: GraphNode) {
  const client = new QueryClient()
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <DetailDrawer node={node} edges={[]} allNodes={[node]} onClose={() => {}} onSelectNode={() => {}} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('DetailDrawer reopen', () => {
  it('offers reopen only for done or stopped intents and met goals', () => {
    const cases: [GraphNode, boolean][] = [
      [{ id: 1, kind: 'intent', state: 'done', payload: {} }, true],
      [{ id: 2, kind: 'intent', state: 'stopped', payload: {} }, true],
      [{ id: 3, kind: 'goal', state: 'met', payload: {} }, true],
      [{ id: 4, kind: 'intent', state: 'open', payload: {} }, false],
      [{ id: 5, kind: 'goal', state: 'open', payload: {} }, false],
      [{ id: 6, kind: 'fact', state: 'confirmed', payload: {} }, false],
    ]
    for (const [node, shown] of cases) {
      const { unmount } = renderDrawer(node)
      expect(!!screen.queryByRole('button', { name: 'Reopen' }), `node ${node.id}`).toBe(shown)
      unmount()
    }
  })

  it('posts the reason and requires one', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 7, kind: 'intent', reopened_goals: [9] }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    renderDrawer({ id: 7, kind: 'intent', state: 'done', payload: { result: 'nothing' } })

    await user.click(screen.getByRole('button', { name: 'Reopen' }))
    const dialog = await screen.findByRole('dialog')
    const submit = within(dialog).getByRole('button', { name: 'Reopen' })
    expect(submit).toBeDisabled()

    await user.type(within(dialog).getByLabelText('Reason'), 'missed the second form')
    await user.click(submit)

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/nodes/7/reopen')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body)).toEqual({ reason: 'missed the second form' })
  })

  it('does not carry an abandoned reason to another node', async () => {
    const user = userEvent.setup()
    const client = new QueryClient()
    const drawer = (node: GraphNode) => (
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <DetailDrawer node={node} edges={[]} allNodes={[node]} onClose={() => {}} onSelectNode={() => {}} />
        </MemoryRouter>
      </QueryClientProvider>
    )
    const { rerender } = render(drawer({ id: 1, kind: 'intent', state: 'done', payload: {} }))
    await user.click(screen.getByRole('button', { name: 'Reopen' }))
    await user.type(within(await screen.findByRole('dialog')).getByLabelText('Reason'), 'half typed')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

    rerender(drawer({ id: 2, kind: 'intent', state: 'done', payload: {} }))
    await user.click(screen.getByRole('button', { name: 'Reopen' }))
    expect(within(await screen.findByRole('dialog')).getByLabelText('Reason')).toHaveValue('')
  })

  it('marks a superseded fact', () => {
    renderDrawer({ id: 8, kind: 'fact', state: 'confirmed', payload: { text: 'f', superseded_by_reopen: 7 } })
    expect(screen.getByText(/Superseded: intent #7 was reopened/)).toBeInTheDocument()
  })
})
