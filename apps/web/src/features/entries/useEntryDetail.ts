import { useState } from 'react'
import { api } from '../../api/client'
import type { CompletedEntryDetail } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export type EntryDetailMode = 'view' | 'edit'

export function useEntryDetail() {
  const { organizationID, setMessage, notifyEntriesChanged } = useWorkspace()
  const [entryDetail, setEntryDetail] = useState<CompletedEntryDetail | null>(null)
  const [mode, setMode] = useState<EntryDetailMode>('view')

  async function openEntryDetail(entryID: string, nextMode: EntryDetailMode = 'view') {
    if (!organizationID) return
    const result = await api<CompletedEntryDetail>(
      `/time-entries/${entryID}?organizationId=${encodeURIComponent(organizationID)}`,
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setMode(nextMode)
      setEntryDetail(result.data)
    }
  }

  async function saveCompletedEntry(input: {
    projectId: string
    ticketIds: string[]
    description: string
    tagIds: string[]
    startedAt?: string
    endedAt?: string
  }) {
    if (!organizationID || !entryDetail) return
    const result = await api<CompletedEntryDetail>(
      `/time-entries/${entryDetail.id}?organizationId=${encodeURIComponent(organizationID)}`,
      { method: 'PATCH', body: JSON.stringify(input) },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      notifyEntriesChanged()
      await openEntryDetail(entryDetail.id, 'view')
      setMessage('Completed entry updated.')
    }
  }

  async function saveTimerEvents(events: { type: string; occurredAt: string }[]) {
    if (!organizationID || !entryDetail) return
    const result = await api<unknown>(
      `/time-entries/${entryDetail.id}/events?organizationId=${encodeURIComponent(organizationID)}`,
      { method: 'PUT', body: JSON.stringify({ events }) },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      notifyEntriesChanged()
      await openEntryDetail(entryDetail.id, 'view')
      setMessage('Timer history updated.')
    }
  }

  async function deleteCompletedEntry() {
    if (
      !organizationID ||
      !entryDetail ||
      !window.confirm('Delete this completed entry? This cannot be undone.')
    )
      return
    const result = await api<{ deleted: boolean }>(
      `/time-entries/${entryDetail.id}?organizationId=${encodeURIComponent(organizationID)}`,
      { method: 'DELETE' },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setEntryDetail(null)
      notifyEntriesChanged()
      setMessage('Completed entry deleted.')
    }
  }

  return {
    entryDetail,
    mode,
    openEntryDetail,
    closeEntryDetail: () => setEntryDetail(null),
    saveCompletedEntry,
    saveTimerEvents,
    deleteCompletedEntry,
  }
}
