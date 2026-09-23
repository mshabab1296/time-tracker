import { useEffect, useState } from 'react'
import { Route, Routes } from 'react-router'
import { api } from '../api/client'
import { PublicShell } from '../components/ui/PublicShell'
import { AuthScreen } from '../features/auth/AuthScreen'
import { ResetPassword } from '../features/auth/ResetPassword'
import { VerifyEmail } from '../features/auth/VerifyEmail'
import { Dashboard } from '../features/dashboard/Dashboard'
import type { User } from '../types/domain'

export default function App() {
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
      <Route path="/reset-password" element={<ResetPassword />} />
      <Route
        path="/*"
        element={
          user ? (
            <Dashboard initialUser={user} onLogout={() => setUser(null)} />
          ) : (
            <AuthScreen onSignedIn={setUser} />
          )
        }
      />
    </Routes>
  )
}
