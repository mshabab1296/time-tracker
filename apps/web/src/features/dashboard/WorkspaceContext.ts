import { createContext, useContext } from 'react'
import type { useWorkspaceState } from './useWorkspaceState'

export type WorkspaceState = ReturnType<typeof useWorkspaceState>

export const WorkspaceContext = createContext<WorkspaceState | null>(null)

export function useWorkspace() {
  const workspace = useContext(WorkspaceContext)
  if (!workspace) throw new Error('Workspace context is unavailable')
  return workspace
}
