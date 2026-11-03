import { useEntries } from './useEntries'
import { entryDateTime, hoursMinutes } from '../../utils/time'
import { EmptyState } from '../../components/ui/EmptyState'
import { IconButton } from '../../components/ui/IconButton'
import { EntryDetailDialog } from './EntryDetailDialog'
import { SingleSelect } from '../../components/ui/SingleSelect'
import { useEntryDetail } from './useEntryDetail'

export function EntriesPage() {
  const {
    completedEntries,
    loading,
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
  } = useEntries()
  const detail = useEntryDetail()

  return (
    <>
      <section className="card page-card">
        <div className="section-heading">
          <div>
            <h2>Completed entries</h2>
            <p>
              {entryUserID === 'all'
                ? "Browse everyone's completed work. Dates use each member's timezone."
                : entryUserID
                  ? "Review this member's completed work. Dates use their timezone."
                  : 'Find and update your completed work. Dates use your timezone.'}
            </p>
          </div>
          <span className="count-chip">{loading ? '…' : (completedEntries?.total ?? 0)}</span>
        </div>
        <div className="entry-filter-grid">
          {isAdmin && (
            <label>
              Member
              <SingleSelect
                label="Member"
                value={entryUserID}
                onValueChange={(value) => {
                  setEntryOffset(0)
                  setEntryUserID(value)
                }}
                options={[
                  { value: 'all', label: 'All members' },
                  { value: '', label: 'My entries' },
                  ...members
                    .filter((member) => member.id !== user.id)
                    .map((member) => ({ value: member.id, label: member.name })),
                ]}
              />
            </label>
          )}
          <label>
            Date
            <input
              type="date"
              value={entryDate}
              onChange={(event) => {
                setEntryOffset(0)
                setEntryDate(event.target.value)
              }}
            />
          </label>
          <label>
            Project
            <SingleSelect
              label="Project"
              value={entryProjectID}
              onValueChange={(value) => {
                setEntryOffset(0)
                setEntryProjectID(value)
              }}
              options={[
                { value: '', label: 'All projects' },
                ...projects.map((project) => ({ value: project.id, label: project.name })),
              ]}
            />
          </label>
          <label>
            Search
            <input
              type="search"
              value={entrySearch}
              onChange={(event) => {
                setEntryOffset(0)
                setEntrySearch(event.target.value)
              }}
              maxLength={200}
              placeholder="Task, project, ticket, or tag"
            />
          </label>
        </div>
        {loading ? (
          <p className="muted" role="status">
            Loading entries…
          </p>
        ) : completedEntries?.items.length ? (
          <div className="entries-table-scroll">
            <table className="entries-table">
              <caption>Completed time entries</caption>
              <thead>
                <tr>
                  <th scope="col">Description</th>
                  <th scope="col">Member</th>
                  <th scope="col">Project</th>
                  <th scope="col">Tickets / tags</th>
                  <th scope="col">Start</th>
                  <th scope="col">End</th>
                  <th scope="col">Duration</th>
                  <th scope="col">Actions</th>
                </tr>
              </thead>
              <tbody>
                {completedEntries.items.map((entry) => (
                  <tr key={entry.id}>
                    <th scope="row" className="entries-description">
                      <strong>{entry.description}</strong>
                      <small>{entry.sourceType === 'TIMER' ? 'Timer' : 'Manual'}</small>
                    </th>
                    <td data-label="Member">
                      {entry.userName}
                      <small>{entry.timezone || selectedTimezone}</small>
                    </td>
                    <td data-label="Project">{entry.projectName}</td>
                    <td data-label="Tickets / tags">
                      {entry.tickets.length > 0 ? (
                        <span>{entry.tickets.map((ticket) => ticket.reference).join(', ')}</span>
                      ) : (
                        <span>—</span>
                      )}
                      {entry.tags.length > 0 && (
                        <span className="entry-tags">
                          {entry.tags.map((tag) => (
                            <em key={tag.id}>{tag.name}</em>
                          ))}
                        </span>
                      )}
                    </td>
                    <td data-label="Start" className="entries-timestamp">
                      <time dateTime={entry.startedAt}>
                        {entryDateTime(entry.startedAt, entry.timezone || selectedTimezone)}
                      </time>
                    </td>
                    <td data-label="End" className="entries-timestamp">
                      <time dateTime={entry.endedAt}>
                        {entryDateTime(entry.endedAt, entry.timezone || selectedTimezone)}
                      </time>
                    </td>
                    <td data-label="Duration" className="entries-duration">
                      {hoursMinutes(entry.durationSeconds)}
                    </td>
                    <td data-label="Actions">
                      <span className="row-actions">
                        <IconButton
                          label={`View ${entry.description}`}
                          icon="◉"
                          onClick={() => void detail.openEntryDetail(entry.id, 'view')}
                        />
                        {entry.userId === user.id && (
                          <IconButton
                            label={`Edit ${entry.description}`}
                            icon="✎"
                            onClick={() => void detail.openEntryDetail(entry.id, 'edit')}
                          />
                        )}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState
            title={
              entryDate || entryProjectID || entrySearch
                ? 'No matching entries'
                : 'No completed entries'
            }
            text={
              entryDate || entryProjectID || entrySearch
                ? 'Try changing the filters or search.'
                : 'Completed timers and manual entries will appear here.'
            }
            compact
          />
        )}
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
