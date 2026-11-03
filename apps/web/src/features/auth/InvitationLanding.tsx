import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { api } from '../../api/client'
import { PublicShell } from '../../components/ui/PublicShell'
import type { User } from '../../types/domain'
import { ForgotPasswordPage } from './ForgotPasswordPage'
import { LoginPage } from './LoginPage'
import { RegisterPage } from './RegisterPage'

export function InvitationLanding({
  user,
  onSignedIn,
  onLogout,
}: {
  user: User | null
  onSignedIn: (user: User) => void
  onLogout: () => void
}) {
  const location = useLocation()
  const navigate = useNavigate()
  const token =
    new URLSearchParams(location.search).get('token') ??
    new URLSearchParams(location.search).get('invitationToken')
  const page = new URLSearchParams(location.search).get('page')
  const attempted = useRef('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!user || !token) return
    const key = `${user.id}:${token}`
    if (attempted.current === key) return
    attempted.current = key
    setError('')
    void (async () => {
      try {
        const result = await api<{ organizationId: string }>('/invitations/accept-token', {
          method: 'POST',
          body: JSON.stringify({ token }),
        })
        if (result.error) {
          setError(result.error.message)
          return
        }
        localStorage.setItem('timetracker.organizationId', result.data.organizationId)
        navigate('/tracker', { replace: true })
      } catch {
        setError('Unable to accept the invitation right now. Please try again later.')
      }
    })()
  }, [user, token, navigate])

  if (!token) {
    return (
      <PublicShell title="Invitation link">
        <p className="notice notice-error" role="alert">
          This invitation link is incomplete.
        </p>
        <Link to="/">Return to TimeTracker</Link>
      </PublicShell>
    )
  }
  if (!user) {
    if (page === 'register') return <RegisterPage invitationToken={token} />
    if (page === 'forgot-password') return <ForgotPasswordPage invitationToken={token} />
    return <LoginPage onSignedIn={onSignedIn} invitationToken={token} />
  }
  return (
    <PublicShell title="Joining organization">
      {error ? (
        <>
          <p className="notice notice-error" role="alert">
            {error}
          </p>
          <button
            className="secondary"
            onClick={() => {
              void api('/auth/logout', { method: 'POST' })
                .catch(() => {})
                .finally(onLogout)
            }}
          >
            Use another account
          </button>
          <p>
            <Link to="/">Continue to TimeTracker</Link>
          </p>
        </>
      ) : (
        <p>Checking your invitation…</p>
      )}
    </PublicShell>
  )
}
