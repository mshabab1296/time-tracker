import { useEffect, useState } from 'react'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'

export function VerifyEmail() {
  const [message, setMessage] = useState('Verifying your email…')
  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get('token')
    api<{ emailVerified: boolean }>('/auth/verify-email', {
      method: 'POST',
      body: JSON.stringify({ token }),
    }).then((result) =>
      setMessage(result.error?.message ?? 'Your email is verified. You can now log in.'),
    )
  }, [])
  return (
    <PublicShell title="Verify email">
      <p className="notice">{message}</p>
      <a href="/">Return to login</a>
    </PublicShell>
  )
}
