import { useEntries } from './useEntries'
import { hoursMinutes } from '../../utils/time'
import { EmptyState } from '../../components/ui/EmptyState'
import { IconButton } from '../../components/ui/IconButton'
import { EntryDetailDialog } from './EntryDetailDialog'
import { SingleSelect } from '../../components/ui/SingleSelect'
import { useEntryDetail } from './useEntryDetail'

export function EntriesPage() {
  const {
    completedEntries,
    isAdmin,
    entryUserID,
    setEntryOffset,
    setEntryUserID,
    members,
    user,
    entryOffset,
  } = useEntries()
  const detail = useEntryDetail()

  return (
    <>
      <section className="card page-card">
        <div className="section-heading">
          <div>
            <h2>Completed entries</h2>
            <p>View work recorded in this workspace.</p>
          </div>
          <span className="count-chip">{completedEntries?.total ?? 0}</span>
        </div>
        {isAdmin && (
          <label className="entry-member-filter">
            Member
            <SingleSelect
              label="Member"
              value={entryUserID}
              onValueChange={(value) => {
                setEntryOffset(0)
                setEntryUserID(value)
              }}
              options={[
                { value: '', label: 'My entries' },
                ...members
                  .filter((member) => member.id !== user.id)
                  .map((member) => ({ value: member.id, label: member.name })),
              ]}
            />
          </label>
        )}
        <div className="entry-list">
          {completedEntries?.items.length ? (
            completedEntries.items.map((entry) => (
              <div className="entry-row completed-entry-row" key={entry.id}>
                <div>
                  <strong>{entry.description}</strong>
                  <small>
                    {entry.projectName} ·{' '}
                    {entry.tickets.length > 0 && (
                      <>{entry.tickets.map((ticket) => ticket.reference).join(', ')} · </>
                    )}
                    {entryUserID && <>{entry.userName} · </>}
                    {new Date(entry.startedAt).toLocaleString()} –{' '}
                    {new Date(entry.endedAt).toLocaleString()} ·{' '}
                    {entry.sourceType === 'TIMER' ? 'Timer' : 'Manual'}
                  </small>
                  {entry.tags.length > 0 && (
                    <span className="entry-tags">
                      {entry.tags.map((tag) => (
                        <em key={tag.id}>{tag.name}</em>
                      ))}
                    </span>
                  )}
                </div>
                <span className="row-actions">
                  <strong>{hoursMinutes(entry.durationSeconds)}</strong>
                  <IconButton
                    label={`View ${entry.description}`}
                    icon="◉"
                    onClick={() => void detail.openEntryDetail(entry.id, 'view')}
                  />
                  <IconButton
                    label={`Edit ${entry.description}`}
                    icon="✎"
                    onClick={() => void detail.openEntryDetail(entry.id, 'edit')}
                  />
                </span>
              </div>
            ))
          ) : (
            <EmptyState
              title="No completed entries"
              text="Completed timers and manual entries will appear here."
              compact
            />
          )}
        </div>
        {completedEntries && completedEntries.total > completedEntries.limit && (
          <div className="pagination">
            <button
              className="secondary"
              disabled={entryOffset === 0}
              onClick={() => setEntryOffset((value) => Math.max(0, value - completedEntries.limit))}
            >
              Previous
            </button>
            <span>
              {entryOffset + 1}–
              {Math.min(entryOffset + completedEntries.limit, completedEntries.total)} of{' '}
              {completedEntries.total}
            </span>
            <button
              className="secondary"
              disabled={entryOffset + completedEntries.limit >= completedEntries.total}
              onClick={() => setEntryOffset((value) => value + completedEntries.limit)}
            >
              Next
            </button>
          </div>
        )}
      </section>
      <EntryDetailDialog detail={detail} />
    </>
  )
}
