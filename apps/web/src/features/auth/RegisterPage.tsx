import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'
import type { User } from '../../types/domain'
import { authPath } from './authPath'
import { PasswordField } from './PasswordField'
import { passwordRequirements, validNewPassword } from './passwordPolicy'

export function RegisterPage({ invitationToken }: { invitationToken?: string }) {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [message, setMessage] = useState('')
  const [messageError, setMessageError] = useState(false)
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!validNewPassword(password)) {
      setMessage(passwordRequirements)
      setMessageError(true)
      return
    }
    if (password !== confirmPassword) {
      setMessage('Passwords do not match.')
      setMessageError(true)
      return
    }
    setBusy(true)
    try {
      const result = await api<User>('/auth/register', {
        method: 'POST',
        body: JSON.stringify({
          email,
          password,
          name,
          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          ...(invitationToken ? { invitationToken } : {}),
        }),
      })
      if (result.error) {
        setMessage(result.error.message)
        setMessageError(true)
      } else if (invitationToken) {
        navigate(authPath('login', invitationToken), {
          replace: true,
          state: { invitationRegistered: true },
        })
      } else {
        setMessage('Check your email to verify your account.')
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
    <PublicShell title="Create your account">
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
        <label>
          Name
          <input
            required
            maxLength={100}
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <PasswordField
          label="Password"
          value={password}
          onChange={setPassword}
          autoComplete="new-password"
          minLength={8}
          hint="At least 8 characters, one uppercase letter, and one number."
        />
        <PasswordField
          label="Confirm password"
          value={confirmPassword}
          onChange={setConfirmPassword}
          autoComplete="new-password"
        />
        <button disabled={busy}>{busy ? 'Working…' : 'Register'}</button>
      </form>
      <div className="links">
        <Link to={authPath('login', invitationToken)}>Log in</Link>
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
