import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'
import { PasswordField } from './PasswordField'
import { passwordRequirements, validNewPassword } from './passwordPolicy'

export function ResetPassword({ onReset }: { onReset: () => void }) {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState('')
  const [messageError, setMessageError] = useState(false)
  const [busy, setBusy] = useState(false)
  const token = new URLSearchParams(window.location.search).get('token')
  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!validNewPassword(password)) {
      setMessage(passwordRequirements)
      setMessageError(true)
      return
    }
    setBusy(true)
    try {
      const result = await api<unknown>('/auth/password-reset/confirm', {
        method: 'POST',
        body: JSON.stringify({ token, password }),
      })
      if (result.error) {
        setMessage(result.error.message)
        setMessageError(true)
      } else {
        onReset()
        navigate('/login', { replace: true, state: { passwordReset: true } })
      }
    } catch {
      setMessage('The service is unavailable. Please try again.')
      setMessageError(true)
    } finally {
      setBusy(false)
    }
  }
  return (
    <PublicShell title="Choose a new password">
      <form className="stack" onSubmit={submit}>
        <PasswordField
          label="New password"
          value={password}
          onChange={setPassword}
          autoComplete="new-password"
          minLength={8}
          hint="At least 8 characters, one uppercase letter, and one number."
        />
        <button disabled={busy}>{busy ? 'Working…' : 'Set password'}</button>
      </form>
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
