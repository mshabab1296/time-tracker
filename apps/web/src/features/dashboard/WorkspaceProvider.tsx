import type { ReactNode } from 'react'
import type { User } from '../../types/domain'
import { WorkspaceContext } from './WorkspaceContext'
import { useWorkspaceState } from './useWorkspaceState'

export function WorkspaceProvider({
  initialUser,
  onLogout,
  children,
}: {
  initialUser: User
  onLogout: () => void
  children: ReactNode
}) {
  const state = useWorkspaceState(initialUser, onLogout)
  return <WorkspaceContext.Provider value={state}>{children}</WorkspaceContext.Provider>
}
