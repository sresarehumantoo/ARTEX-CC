import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { RunningIntents } from '../Dashboard'

const now = Date.parse('2026-11-25T12:00:00Z')

describe('RunningIntents', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(now)
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the claim holder and time remaining', () => {
    render(
      <RunningIntents
        intents={[
          { id: 1, state: 'running', summary: 'Fuzz search', claimed_by: 'qa-alice', claim_expires_at: '2026-11-25T12:29:30Z' },
        ]}
      />,
    )
    expect(screen.getByText('Fuzz search')).toBeInTheDocument()
    expect(screen.getByText('qa-alice')).toHaveAttribute('title', 'qa-alice')
    expect(screen.getByText(/29m left/)).toBeInTheDocument()
  })

  it('shortens chat holders and keeps the full name in a tooltip', () => {
    const full = 'chat/7ef7a2c1-4b9d-4e8a-9c3f-2d61b0e5a9f4'
    render(
      <RunningIntents
        intents={[{ id: 2, state: 'running', summary: 'Probe API', claimed_by: full, claim_expires_at: '2026-11-25T13:05:00Z' }]}
      />,
    )
    expect(screen.getByText('chat/7ef7a2c1…')).toHaveAttribute('title', full)
    expect(screen.getByText(/1h 5m left/)).toBeInTheDocument()
  })

  it('hints that a lapsed claim is no longer held', () => {
    render(
      <RunningIntents
        intents={[{ id: 3, state: 'running', summary: 'Read users table', claim_expired: true, previous_owner: 'h-3f9a2c' }]}
      />,
    )
    expect(screen.getByText(/claim lapsed/)).toBeInTheDocument()
    expect(screen.getByText('h-3f9a2c')).toHaveAttribute('title', 'h-3f9a2c')
    expect(screen.queryByText(/left/)).not.toBeInTheDocument()
  })
})
