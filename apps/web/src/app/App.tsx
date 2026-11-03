import { useEffect, useState } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router'
import { api } from '../api/client'
import { PublicShell } from '../components/ui/PublicShell'
import { ForgotPasswordPage } from '../features/auth/ForgotPasswordPage'
import { LoginPage } from '../features/auth/LoginPage'
import { RegisterPage } from '../features/auth/RegisterPage'
import { ResetPassword } from '../features/auth/ResetPassword'
import { VerifyEmail } from '../features/auth/VerifyEmail'
import { InvitationLanding } from '../features/auth/InvitationLanding'
import { Dashboard } from '../features/dashboard/Dashboard'
import type { User } from '../types/domain'

export default function App() {
  const location = useLocation()
  const [user, setUser] = useState<User | null>(null)
  const [checked, setChecked] = useState(false)

  useEffect(() => {
    api<User>('/auth/me')
      .then((result) => {
        if (!result.error) setUser(result.data)
      })
      .finally(() => setChecked(true))
  }, [])

  if (!checked)
    return (
      <PublicShell title="TimeTracker">
        <p>Loading…</p>
      </PublicShell>
    )

  return (
    <Routes>
      <Route path="/verify-email" element={<VerifyEmail />} />
      <Route path="/reset-password" element={<ResetPassword onReset={() => setUser(null)} />} />
      <Route
        path="/login"
        element={user ? <Navigate to="/tracker" replace /> : <LoginPage onSignedIn={setUser} />}
      />
      <Route
        path="/register"
        element={user ? <Navigate to="/tracker" replace /> : <RegisterPage />}
      />
      <Route
        path="/forgot-password"
        element={user ? <Navigate to="/tracker" replace /> : <ForgotPasswordPage />}
      />
      <Route
        path="/invite"
        element={
          <InvitationLanding user={user} onSignedIn={setUser} onLogout={() => setUser(null)} />
        }
      />
      <Route
        path="/*"
        element={
          new URLSearchParams(location.search).has('invitationToken') ? (
            <InvitationLanding user={user} onSignedIn={setUser} onLogout={() => setUser(null)} />
          ) : user ? (
            <Dashboard initialUser={user} onLogout={() => setUser(null)} />
          ) : (
            <Navigate to="/login" replace state={location.state} />
          )
        }
      />
    </Routes>
  )
}
