import type { ReactNode } from 'react'
import { NavLink } from 'react-router'
import { pages } from '../../app/navigation'
import { Toast } from '../ui/Toast'
import { SingleSelect } from '../ui/SingleSelect'
import type { Organization, Page, User } from '../../types/domain'
import type { Notice } from '../../types/notice'

export function DashboardLayout({
  user,
  organizations,
  organizationID,
  page,
  isOrganizationSetup = false,
  notice,
  children,
  headingAction,
  onOrganizationChange,
  onDismissNotice,
  onLogout,
}: {
  user: User
  organizations: Organization[]
  organizationID: string
  page: Page
  isOrganizationSetup?: boolean
  notice: Notice | null
  children: ReactNode
  headingAction?: ReactNode
  onOrganizationChange: (id: string) => void
  onDismissNotice: () => void
  onLogout: () => void
}) {
  const organization = organizations.find((item) => item.id === organizationID)
  const pageDetails = isOrganizationSetup
    ? page === 'team'
      ? { title: 'Your invitations', description: 'Join an organization that invited you.' }
      : {
          title: 'Create your organization',
          description: 'Set up a workspace to start tracking time with your team.',
        }
    : pages[page]

  return (
    <div className="dashboard-shell">
      <aside className="side-nav">
        <div className="brand">
          <span className="brand-mark">T</span>
          <span>TimeTracker</span>
        </div>
        <nav aria-label="Primary navigation">
          {(isOrganizationSetup
            ? (['team', 'settings'] as Page[])
            : (Object.keys(pages) as Page[])
          ).map((item) => (
            <NavLink
              to={`/${item}`}
              className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}
              key={item}
            >
              <span>{pages[item].icon}</span>
              {pages[item].title}
            </NavLink>
          ))}
        </nav>
        <div className="side-footer">
          <span className="avatar">{user.name.slice(0, 1).toUpperCase()}</span>
          <span>{user.name}</span>
        </div>
      </aside>
      <div className="dashboard-main">
        <header className="topbar">
          {!isOrganizationSetup && (
            <label className="organization-picker">
              <span>Workspace</span>
              <SingleSelect
                label="Workspace"
                value={organizationID}
                onValueChange={onOrganizationChange}
                options={[
                  { value: '', label: 'Select workspace' },
                  ...organizations.map((item) => ({
                    value: item.id,
                    label: `${item.name} · ${item.role}`,
                  })),
                ]}
              />
            </label>
          )}
          <div className="topbar-user">
            <span>{user.name}</span>
            <button className="ghost" onClick={onLogout}>
              Log out
            </button>
          </div>
        </header>
        <main className="page-content">
          <div className="page-heading">
            <div>
              <p className="eyebrow">
                {isOrganizationSetup ? 'Getting started' : (organization?.name ?? 'Workspace')}
              </p>
              <h1>{pageDetails.title}</h1>
              <p>{pageDetails.description}</p>
            </div>
            {headingAction}
          </div>
          {children}
        </main>
      </div>
      {notice && <Toast notice={notice} onDismiss={onDismissNotice} />}
    </div>
  )
}
