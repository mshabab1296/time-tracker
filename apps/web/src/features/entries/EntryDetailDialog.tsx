import { useWorkspace } from '../dashboard/WorkspaceContext'
import { EntryDetailModal } from './EntryDetailModal'
import type { useEntryDetail } from './useEntryDetail'

export function EntryDetailDialog({ detail }: { detail: ReturnType<typeof useEntryDetail> }) {
  const { user, isAdmin, organizationID, projects, tags } = useWorkspace()
  const entry = detail.entryDetail
  if (!entry) return null

  return (
    <EntryDetailModal
      key={`${entry.id}:${detail.mode}`}
      entry={entry}
      initialMode={detail.mode}
      projects={projects}
      tags={tags}
      organizationID={organizationID}
      canManage={isAdmin || entry.userId === user.id}
      onClose={detail.closeEntryDetail}
      onSave={(input) => void detail.saveCompletedEntry(input)}
      onSaveEvents={(events) => void detail.saveTimerEvents(events)}
      onDelete={() => void detail.deleteCompletedEntry()}
    />
  )
}
