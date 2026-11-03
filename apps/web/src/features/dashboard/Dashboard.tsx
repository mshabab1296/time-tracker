import { Link, Navigate, Route, Routes, useLocation } from 'react-router'
import { pages as pageDetails } from '../../app/navigation'
import { DashboardLayout } from '../../components/layout/DashboardLayout'
import { SingleSelect } from '../../components/ui/SingleSelect'
import type { Page, User } from '../../types/domain'
import { EntriesPage } from '../entries/EntriesPage'
import { ProjectsPage } from '../projects/ProjectsPage'
import { ReportsPage } from '../reports/ReportsPage'
import { SettingsPage } from '../settings/SettingsPage'
import { TagsPage } from '../tags/TagsPage'
import { TicketsPage } from '../tickets/TicketsPage'
import { TeamPage } from '../team/TeamPage'
import { TrackerPage } from '../tracker/TrackerPage'
import { useWorkspace } from './WorkspaceContext'
import { WorkspaceProvider } from './WorkspaceProvider'

const pageComponents = {
  tracker: TrackerPage,
  entries: EntriesPage,
  reports: ReportsPage,
  projects: ProjectsPage,
  tickets: TicketsPage,
  tags: TagsPage,
  team: TeamPage,
  settings: SettingsPage,
}

function DashboardContent() {
  const { pathname } = useLocation()
  const routeName = pathname.slice(1)
  const page: Page = routeName in pageDetails ? (routeName as Page) : 'tracker'
  const {
    user,
    organizations,
    organizationsLoaded,
    organizationID,
    organization,
    isAdmin,
    members,
    trackerUserID,
    setTrackerUserID,
    incoming,
    notice,
    dismissNotice,
    selectOrganization,
    logout,
  } = useWorkspace()
  const isOrganizationSetup = organizationsLoaded && !organization

  if (!organizationsLoaded) return <p>Loading workspaces…</p>

  return (
    <DashboardLayout
      user={user}
      organizations={organizations}
      organizationID={organizationID}
      page={page}
      headingAction={
        page === 'tracker' && isAdmin && organization ? (
          <label className="tracker-member-picker">
            Member
            <SingleSelect
              label="Member"
              value={trackerUserID}
              onValueChange={setTrackerUserID}
              options={[
                { value: '', label: 'Me' },
                ...members
                  .filter((member) => member.id !== user.id)
                  .map((member) => ({ value: member.id, label: member.name })),
              ]}
            />
          </label>
        ) : undefined
      }
      isOrganizationSetup={isOrganizationSetup}
      notice={notice}
      onDismissNotice={dismissNotice}
      onOrganizationChange={selectOrganization}
      onLogout={() => void logout()}
    >
      {incoming.length > 0 && (
        <p className="notice" role="status">
          You have {incoming.length} pending organization{' '}
          {incoming.length === 1 ? 'invitation' : 'invitations'}.{' '}
          <Link to="/team">Review invitations</Link>
        </p>
      )}
      {organization ? (
        <Routes key={organizationID}>
          <Route index element={<Navigate to="/tracker" replace />} />
          {(Object.keys(pageComponents) as Page[]).map((name) => {
            const PageComponent = pageComponents[name]
            return (
              <Route
                key={name}
                path={name}
                element={<PageComponent key={name === 'tracker' ? trackerUserID : name} />}
              />
            )
          })}
          <Route path="*" element={<Navigate to="/tracker" replace />} />
        </Routes>
      ) : (
        <>
          {pathname !== '/settings' && pathname !== '/team' && (
            <Navigate to={incoming.length ? '/team' : '/settings'} replace />
          )}
          {pathname === '/team' ? <TeamPage /> : <SettingsPage setup />}
        </>
      )}
    </DashboardLayout>
  )
}

export function Dashboard({ initialUser, onLogout }: { initialUser: User; onLogout: () => void }) {
  return (
    <WorkspaceProvider initialUser={initialUser} onLogout={onLogout}>
      <DashboardContent />
    </WorkspaceProvider>
  )
}
