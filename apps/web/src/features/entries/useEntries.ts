import { useCallback, useEffect, useState } from 'react'
import { api } from '../../api/client'
import type { CompletedEntryPage } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useEntries() {
  const { organizationID, isAdmin, members, user, entriesRevision } = useWorkspace()
  const [completedEntries, setCompletedEntries] = useState<CompletedEntryPage | null>(null)

  const [entryUserID, setEntryUserID] = useState('')

  const [entryOffset, setEntryOffset] = useState(0)

  const loadCompletedEntries = useCallback(async () => {
    if (!organizationID) {
      setCompletedEntries(null)
      return
    }
    const target = entryUserID ? `&userId=${encodeURIComponent(entryUserID)}` : ''
    const result = await api<CompletedEntryPage>(
      `/time-entries?organizationId=${encodeURIComponent(organizationID)}&limit=25&offset=${entryOffset}${target}`,
    )
    if (!result.error) setCompletedEntries(result.data)
  }, [organizationID, entryOffset, entryUserID])

  useEffect(() => {
    void loadCompletedEntries()
  }, [loadCompletedEntries, entriesRevision])
  return {
    completedEntries,
    isAdmin,
    entryUserID,
    setEntryOffset,
    setEntryUserID,
    members,
    user,
    entryOffset,
  }
}
