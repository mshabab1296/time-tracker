import { useState, type FormEvent } from 'react'
import { Link, useLocation } from 'react-router'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'
import type { User } from '../../types/domain'
import { authPath } from './authPath'
import { PasswordField } from './PasswordField'

export function LoginPage({
  onSignedIn,
  invitationToken,
}: {
  onSignedIn: (user: User) => void
  invitationToken?: string
}) {
  const location = useLocation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState(
    location.state?.passwordReset
      ? 'Password changed. You can now log in.'
      : location.state?.invitationRegistered
        ? 'Your account is ready and your email is verified. Log in to join the organization.'
        : '',
  )
  const [messageError, setMessageError] = useState(false)
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      const result = await api<User>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      if (result.error) {
        setMessage(result.error.message)
        setMessageError(true)
      } else onSignedIn(result.data)
    } catch {
      setMessage('The service is unavailable. Please try again.')
      setMessageError(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <PublicShell title="Welcome back">
      {invitationToken && (
        <p className="notice">
          Sign in or create an account with the invited email address to join the organization.
        </p>
      )}
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
        <PasswordField
          label="Password"
          value={password}
          onChange={setPassword}
          autoComplete="current-password"
        />
        <button disabled={busy}>{busy ? 'Working…' : 'Log in'}</button>
      </form>
      <div className="links">
        <Link to={authPath('register', invitationToken)}>Create an account</Link>
        <Link to={authPath('forgot-password', invitationToken)}>Forgot password?</Link>
      </div>
      {message && (
        <p
          className={`notice${messageError ? ' notice-error' : ''}`}
          role={messageError ? 'alert' : 'status'}
        >
          {message}
        </p>
      )}
    </PublicShell>
  )
}
