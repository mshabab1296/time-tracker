import { useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'

export function ResetPassword() {
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState('')
  const token = new URLSearchParams(window.location.search).get('token')
  async function submit(event: FormEvent) {
    event.preventDefault()
    const result = await api<unknown>('/auth/password-reset/confirm', {
      method: 'POST',
      body: JSON.stringify({ token, password }),
    })
    setMessage(result.error?.message ?? 'Password changed. You can now log in.')
  }
  return (
    <PublicShell title="Choose a new password">
      <form className="stack" onSubmit={submit}>
        <label>
          New password
          <input
            type="password"
            minLength={12}
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        <button>Set password</button>
      </form>
      {message && <p className="notice">{message}</p>}
    </PublicShell>
  )
}
