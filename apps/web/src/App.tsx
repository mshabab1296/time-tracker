import { FormEvent, ReactNode, useCallback, useEffect, useState } from 'react'

type User = { id: string; email: string; name: string; timezone: string; emailVerified: boolean }
type Organization = { id: string; name: string; timezone: string; role: 'ADMIN' | 'MEMBER' }
type APIResponse<T> = { data: T; error: { code: string; message: string } | null }

function csrfToken() {
  return document.cookie.split('; ').find(cookie => cookie.startsWith('timetracker_csrf='))?.split('=')[1] ?? ''
}

async function api<T>(path: string, options: RequestInit = {}): Promise<APIResponse<T>> {
  const headers = new Headers(options.headers)
  if (options.body) headers.set('Content-Type', 'application/json')
  if (['POST', 'PATCH', 'PUT', 'DELETE'].includes(options.method ?? 'GET') && csrfToken()) headers.set('X-CSRF-Token', csrfToken())
  const response = await fetch(`/api/v1${path}`, { ...options, headers, credentials: 'include' })
  return response.json() as Promise<APIResponse<T>>
}

function AuthScreen({ onSignedIn }: { onSignedIn: (user: User) => void }) {
  const [mode, setMode] = useState<'register' | 'login' | 'forgot'>('register')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setMessage('')
    const path = mode === 'register' ? '/auth/register' : mode === 'login' ? '/auth/login' : '/auth/password-reset/request'
    const body = mode === 'register' ? { email, password, name, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone } : mode === 'login' ? { email, password } : { email }
    try {
      const result = await api<User>(path, { method: 'POST', body: JSON.stringify(body) })
      if (result.error) setMessage(result.error.message)
      else if (mode === 'login') onSignedIn(result.data)
      else setMessage(mode === 'register' ? 'Check your email (Mailpit locally) to verify your account.' : 'If that email has an account, a reset link has been sent.')
    } catch { setMessage('The service is unavailable. Please try again.') } finally { setBusy(false) }
  }

  const title = mode === 'register' ? 'Create your account' : mode === 'login' ? 'Welcome back' : 'Reset your password'
  return <PublicShell title={title}><form onSubmit={submit} className="stack">
    <label>Email<input type="email" required value={email} onChange={event => setEmail(event.target.value)} /></label>
    {mode === 'register' && <label>Name<input required maxLength={100} value={name} onChange={event => setName(event.target.value)} /></label>}
    {mode !== 'forgot' && <label>Password<input type="password" minLength={12} required value={password} onChange={event => setPassword(event.target.value)} /></label>}
    <button disabled={busy} type="submit">{busy ? 'Working…' : mode === 'register' ? 'Register' : mode === 'login' ? 'Log in' : 'Send reset link'}</button>
  </form><div className="links">
    {mode !== 'login' && <button className="link" onClick={() => { setMode('login'); setMessage('') }}>Log in</button>}
    {mode !== 'register' && <button className="link" onClick={() => { setMode('register'); setMessage('') }}>Create an account</button>}
    {mode !== 'forgot' && <button className="link" onClick={() => { setMode('forgot'); setMessage('') }}>Forgot password?</button>}
  </div>{message && <p role="status" className="notice">{message}</p>}</PublicShell>
}

function VerifyEmail() {
  const [message, setMessage] = useState('Verifying your email…')
  useEffect(() => { const token = new URLSearchParams(window.location.search).get('token'); if (!token) { setMessage('This verification link is missing its token.'); return }; api<{ emailVerified: boolean }>('/auth/verify-email', { method: 'POST', body: JSON.stringify({ token }) }).then(result => setMessage(result.error?.message ?? 'Your email is verified. You can now log in.')).catch(() => setMessage('The service is unavailable. Please try again.')) }, [])
  return <PublicShell title="Verify email"><p className="notice">{message}</p><a href="/">Return to login</a></PublicShell>
}

function ResetPassword() {
  const [password, setPassword] = useState(''); const [message, setMessage] = useState(''); const token = new URLSearchParams(window.location.search).get('token')
  async function submit(event: FormEvent) { event.preventDefault(); const result = await api<{ passwordReset: boolean }>('/auth/password-reset/confirm', { method: 'POST', body: JSON.stringify({ token, password }) }); setMessage(result.error?.message ?? 'Password changed. You can now log in.') }
  return <PublicShell title="Choose a new password"><form onSubmit={submit} className="stack"><label>New password<input type="password" minLength={12} required value={password} onChange={event => setPassword(event.target.value)} /></label><button>Set password</button></form>{message && <p className="notice" role="status">{message}</p>}<a href="/">Return to login</a></PublicShell>
}

function Dashboard({ initialUser, onLogout }: { initialUser: User; onLogout: () => void }) {
  const [user, setUser] = useState(initialUser); const [organizations, setOrganizations] = useState<Organization[]>([]); const [selectedID, setSelectedID] = useState(localStorage.getItem('timetracker.organizationId') ?? ''); const [name, setName] = useState(initialUser.name); const [organizationName, setOrganizationName] = useState(''); const [message, setMessage] = useState('')
  const loadOrganizations = useCallback(async () => { const result = await api<Organization[]>('/organizations'); if (result.error) { setMessage(result.error.message); return }; setOrganizations(result.data); if (!selectedID && result.data[0]) { setSelectedID(result.data[0].id); localStorage.setItem('timetracker.organizationId', result.data[0].id) } }, [selectedID])
  useEffect(() => { void loadOrganizations() }, [loadOrganizations])
  async function saveProfile(event: FormEvent) { event.preventDefault(); const result = await api<User>('/profile', { method: 'PATCH', body: JSON.stringify({ name }) }); if (result.error) setMessage(result.error.message); else { setUser(result.data); setMessage('Profile updated.') } }
  async function createOrganization(event: FormEvent) { event.preventDefault(); const result = await api<Organization>('/organizations', { method: 'POST', body: JSON.stringify({ name: organizationName, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone }) }); if (result.error) setMessage(result.error.message); else { setOrganizationName(''); await loadOrganizations(); setSelectedID(result.data.id); localStorage.setItem('timetracker.organizationId', result.data.id); setMessage('Organization created.') } }
  async function logout() { const result = await api<{ loggedOut: boolean }>('/auth/logout', { method: 'POST' }); if (result.error) setMessage(result.error.message); else onLogout() }
  return <main className="dashboard"><header><div><p className="eyebrow">TimeTracker V1</p><h1>Welcome, {user.name}</h1></div><button className="secondary" onClick={logout}>Log out</button></header><section className="card"><h2>Organization context</h2><label>Selected organization<select value={selectedID} onChange={event => { setSelectedID(event.target.value); localStorage.setItem('timetracker.organizationId', event.target.value) }}><option value="">Select an organization</option>{organizations.map(organization => <option key={organization.id} value={organization.id}>{organization.name} · {organization.role}</option>)}</select></label><form onSubmit={createOrganization} className="inline-form"><input aria-label="Organization name" placeholder="New organization name" required maxLength={100} value={organizationName} onChange={event => setOrganizationName(event.target.value)} /><button>Create organization</button></form></section><section className="card"><h2>Your profile</h2><p>{user.email} · {user.timezone}</p><form onSubmit={saveProfile} className="inline-form"><input aria-label="Name" required maxLength={100} value={name} onChange={event => setName(event.target.value)} /><button>Save profile</button></form></section>{message && <p role="status" className="notice">{message}</p>}</main>
}

function PublicShell({ title, children }: { title: string; children: ReactNode }) { return <main className="app-shell"><section className="card auth-card"><p className="eyebrow">TimeTracker V1</p><h1>{title}</h1>{children}</section></main> }

function App() {
  const [user, setUser] = useState<User | null>(null); const [checked, setChecked] = useState(false)
  useEffect(() => { api<User>('/auth/me').then(result => { if (!result.error) setUser(result.data) }).finally(() => setChecked(true)) }, [])
  if (!checked) return <PublicShell title="TimeTracker"><p>Loading…</p></PublicShell>
  if (window.location.pathname === '/verify-email') return <VerifyEmail />
  if (window.location.pathname === '/reset-password') return <ResetPassword />
  return user ? <Dashboard initialUser={user} onLogout={() => setUser(null)} /> : <AuthScreen onSignedIn={setUser} />
}

export default App
