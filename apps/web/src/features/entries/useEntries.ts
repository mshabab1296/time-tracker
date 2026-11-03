import { useEffect, useState } from 'react'
import { api } from '../../api/client'
import type { CompletedEntryPage } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useEntries() {
  const { organizationID, isAdmin, members, projects, user, entriesRevision, setMessage } =
    useWorkspace()
  const [completedEntries, setCompletedEntries] = useState<CompletedEntryPage | null>(null)
  const [entryUserID, setEntryUserID] = useState('')
  const [entryDate, setEntryDate] = useState('')
  const [entryProjectID, setEntryProjectID] = useState('')
  const [entrySearch, setEntrySearch] = useState('')
  const [appliedSearch, setAppliedSearch] = useState('')
  const [entryOffset, setEntryOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const selectedTimezone =
    completedEntries?.timezone ??
    members.find((member) => member.id === entryUserID)?.timezone ??
    user.timezone

  useEffect(() => {
    const timeout = window.setTimeout(() => setAppliedSearch(entrySearch.trim()), 300)
    return () => window.clearTimeout(timeout)
  }, [entrySearch])

  useEffect(() => {
    if (!organizationID) {
      setCompletedEntries(null)
      return
    }
    const controller = new AbortController()
    const parameters = new URLSearchParams({
      organizationId: organizationID,
      limit: '25',
      offset: String(entryOffset),
    })
    if (entryUserID) parameters.set('userId', entryUserID)
    if (entryDate) parameters.set('date', entryDate)
    if (entryProjectID) parameters.set('projectId', entryProjectID)
    if (appliedSearch) parameters.set('search', appliedSearch)
    setLoading(true)
    setCompletedEntries(null)
    void api<CompletedEntryPage>(`/time-entries?${parameters}`, { signal: controller.signal })
      .then((result) => {
        if (controller.signal.aborted) return
        if (result.error) setMessage(result.error.message, 'error')
        else setCompletedEntries(result.data)
      })
      .catch(() => {
        if (!controller.signal.aborted) setMessage('Unable to load completed entries.', 'error')
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [
    organizationID,
    entryUserID,
    entryDate,
    entryProjectID,
    appliedSearch,
    entryOffset,
    entriesRevision,
    setMessage,
  ])

  return {
    completedEntries,
    loading: loading || entrySearch.trim() !== appliedSearch,
    isAdmin,
    entryUserID,
    setEntryOffset,
    setEntryUserID,
    entryDate,
    setEntryDate,
    entryProjectID,
    setEntryProjectID,
    entrySearch,
    setEntrySearch,
    members,
    projects,
    user,
    selectedTimezone,
    entryOffset,
  }
}
