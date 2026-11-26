import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { AuthShell } from '../components/AuthShell'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { checkAuthStatus, initAuth } from '../lib/auth'

const MIN_LENGTH = 8

export default function Setup() {
  const navigate = useNavigate()
  const [setupToken, setSetupToken] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(true)

  useEffect(() => {
    checkAuthStatus()
      .then(({ initialized }) => {
        if (initialized) navigate('/login', { replace: true })
      })
      .catch(() => setError('Unable to reach the server'))
      .finally(() => setChecking(false))
  }, [navigate])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (password.length < MIN_LENGTH) {
      setError(`Password must be at least ${MIN_LENGTH} characters`)
      return
    }
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }
    setLoading(true)
    setError('')
    try {
      await initAuth(password, setupToken.trim())
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Setup failed')
    } finally {
      setLoading(false)
    }
  }

  if (checking) {
    return (
      <div role="status" className="flex min-h-dvh items-center justify-center text-muted-foreground">
        Checking setup status...
      </div>
    )
  }

  return (
    <AuthShell title="Set a password" description="Choose a password to protect this ARTEX-CC instance.">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="setup-token">Setup token</Label>
          <Input
            id="setup-token"
            value={setupToken}
            onChange={e => setSetupToken(e.target.value)}
            aria-describedby="setup-token-hint"
            autoFocus
            autoComplete="off"
            spellCheck={false}
          />
          <p id="setup-token-hint" className="text-xs text-muted-foreground">
            Printed in the server logs at startup (<code>docker compose logs artex</code> for Docker).
          </p>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="password">Set password</Label>
          <Input
            id="password"
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder={`At least ${MIN_LENGTH} characters`}
            minLength={MIN_LENGTH}
            autoComplete="new-password"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="confirm">Confirm password</Label>
          <Input
            id="confirm"
            type="password"
            value={confirm}
            onChange={e => setConfirm(e.target.value)}
            placeholder="Repeat password"
            autoComplete="new-password"
          />
        </div>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <Button type="submit" className="w-full" disabled={loading || !setupToken.trim() || !password || !confirm}>
          {loading ? 'Saving...' : 'Set password'}
        </Button>
      </form>
    </AuthShell>
  )
}
