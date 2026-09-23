import { Navigate, Route, Routes, useLocation } from 'react-router'
import { pages as pageDetails } from '../../app/navigation'
import { DashboardLayout } from '../../components/layout/DashboardLayout'
import { EmptyState } from '../../components/ui/EmptyState'
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
    organizationID,
    organization,
    notice,
    dismissNotice,
    selectOrganization,
    logout,
  } = useWorkspace()

  return (
    <DashboardLayout
      user={user}
      organizations={organizations}
      organizationID={organizationID}
      page={page}
      notice={notice}
      onDismissNotice={dismissNotice}
      onOrganizationChange={selectOrganization}
      onLogout={() => void logout()}
    >
      {organization ? (
        <Routes key={organizationID}>
          <Route index element={<Navigate to="/tracker" replace />} />
          {(Object.keys(pageComponents) as Page[]).map((name) => {
            const PageComponent = pageComponents[name]
            return <Route key={name} path={name} element={<PageComponent />} />
          })}
          <Route path="*" element={<Navigate to="/tracker" replace />} />
        </Routes>
      ) : (
        <EmptyState
          title="Create or select an organization"
          text="Organizations keep projects, tags, Members, and time entries separate."
        />
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
