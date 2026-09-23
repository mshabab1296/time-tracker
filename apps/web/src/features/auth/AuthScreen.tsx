import { useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'
import type { User } from '../../types/domain'

export function AuthScreen({ onSignedIn }: { onSignedIn: (user: User) => void }) {
  const [mode, setMode] = useState<'register' | 'login' | 'forgot'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    const path =
      mode === 'register'
        ? '/auth/register'
        : mode === 'login'
          ? '/auth/login'
          : '/auth/password-reset/request'
    const body =
      mode === 'register'
        ? { email, password, name, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone }
        : mode === 'login'
          ? { email, password }
          : { email }
    try {
      const result = await api<User>(path, { method: 'POST', body: JSON.stringify(body) })
      if (result.error) setMessage(result.error.message)
      else if (mode === 'login') onSignedIn(result.data)
      else
        setMessage(
          mode === 'register'
            ? 'Check your email to verify your account.'
            : 'If that account exists, a reset link was sent.',
        )
    } catch {
      setMessage('The service is unavailable. Please try again.')
    } finally {
      setBusy(false)
    }
  }
  const title =
    mode === 'register'
      ? 'Create your account'
      : mode === 'forgot'
        ? 'Reset your password'
        : 'Welcome back'
  return (
    <PublicShell title={title}>
      <form onSubmit={submit} className="stack">
        <label>
          Email
          <input
            type="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </label>
        {mode === 'register' && (
          <label>
            Name
            <input
              required
              maxLength={100}
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
        )}
        {mode !== 'forgot' && (
          <label>
            Password
            <input
              type="password"
              minLength={12}
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
        )}
        <button disabled={busy}>
          {busy
            ? 'Working…'
            : mode === 'login'
              ? 'Log in'
              : mode === 'register'
                ? 'Register'
                : 'Send reset link'}
        </button>
      </form>
      <div className="links">
        <button className="link" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
          {mode === 'login' ? 'Create an account' : 'Log in'}
        </button>
        {mode !== 'forgot' && (
          <button className="link" onClick={() => setMode('forgot')}>
            Forgot password?
          </button>
        )}
      </div>
      {message && <p className="notice">{message}</p>}
    </PublicShell>
  )
}
