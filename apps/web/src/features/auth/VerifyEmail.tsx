import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'

export function VerifyEmail() {
  const [message, setMessage] = useState('Verifying your email…')
  const [isError, setIsError] = useState(false)
  const started = useRef(false)
  useEffect(() => {
    if (started.current) return
    started.current = true
    const token = new URLSearchParams(window.location.search).get('token')
    api<{ emailVerified: boolean }>('/auth/verify-email', {
      method: 'POST',
      body: JSON.stringify({ token }),
    })
      .then((result) => {
        setIsError(Boolean(result.error))
        setMessage(result.error?.message ?? 'Your email is verified. You can now log in.')
      })
      .catch(() => {
        setIsError(true)
        setMessage('Unable to verify your email right now. Please try again later.')
      })
  }, [])
  return (
    <PublicShell title="Verify email">
      <p className={`notice${isError ? ' notice-error' : ''}`} role={isError ? 'alert' : 'status'}>
        {message}
      </p>
      <a href="/">Return to login</a>
    </PublicShell>
  )
}
