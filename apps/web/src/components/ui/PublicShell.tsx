import type { ReactNode } from 'react'

export function PublicShell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="app-shell">
      <section className="card auth-card">
        <p className="eyebrow">TimeTracker V1</p>
        <h1>{title}</h1>
        {children}
      </section>
    </main>
  )
}
