import { useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'
import { authPath } from './authPath'

export function ForgotPasswordPage({ invitationToken }: { invitationToken?: string }) {
  const [email, setEmail] = useState('')
  const [message, setMessage] = useState('')
  const [messageError, setMessageError] = useState(false)
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      const result = await api<unknown>('/auth/password-reset/request', {
        method: 'POST',
        body: JSON.stringify({ email }),
      })
      if (result.error) {
        setMessage(result.error.message)
        setMessageError(true)
      } else {
        setEmail('')
        setMessage('If that account exists, a reset link was sent.')
        setMessageError(false)
      }
    } catch {
      setMessage('The service is unavailable. Please try again.')
      setMessageError(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <PublicShell title="Reset your password">
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
        <button disabled={busy}>{busy ? 'Working…' : 'Send reset link'}</button>
      </form>
      <div className="links">
        <Link to={authPath('login', invitationToken)}>Log in</Link>
        <Link to={authPath('register', invitationToken)}>Create an account</Link>
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
