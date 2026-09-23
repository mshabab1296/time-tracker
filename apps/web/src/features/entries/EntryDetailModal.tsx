import { useState, type FormEvent } from 'react'
import { IconButton } from '../../components/ui/IconButton'
import { ModalHeader } from '../../components/ui/ModalHeader'
import { ModalFooter } from '../../components/ui/ModalFooter'
import { TagMultiSelect } from '../../components/ui/TagMultiSelect'
import { SingleSelect } from '../../components/ui/SingleSelect'
import type { CompletedEntryDetail, Project, Tag } from '../../types/domain'
import { hoursMinutes, localDateTimeInput, localDateTimeSecondInput } from '../../utils/time'
import { TicketPicker } from '../tickets/TicketPicker'

export function EntryDetailModal({
  entry,
  initialMode,
  projects,
  tags,
  organizationID,
  canManage,
  onClose,
  onSave,
  onSaveEvents,
  onDelete,
}: {
  entry: CompletedEntryDetail
  initialMode: 'view' | 'edit'
  projects: Project[]
  tags: Tag[]
  organizationID: string
  canManage: boolean
  onClose: () => void
  onSave: (input: {
    projectId: string
    ticketIds: string[]
    description: string
    tagIds: string[]
    startedAt?: string
    endedAt?: string
  }) => void
  onSaveEvents: (events: { type: string; occurredAt: string }[]) => void
  onDelete: () => void
}) {
  const [editing, setEditing] = useState(initialMode === 'edit')
  const [projectID, setProjectID] = useState(entry.projectId)
  const [ticketIDs, setTicketIDs] = useState<string[]>(entry.tickets.map((ticket) => ticket.id))
  const [description, setDescription] = useState(entry.description)
  const [tagIDs, setTagIDs] = useState(entry.tags.map((tag) => tag.id))
  const [startedAt, setStartedAt] = useState(() => localDateTimeInput(new Date(entry.startedAt)))
  const [endedAt, setEndedAt] = useState(() => localDateTimeInput(new Date(entry.endedAt)))
  function toggle(id: string) {
    setTagIDs((values) =>
      values.includes(id) ? values.filter((value) => value !== id) : [...values, id],
    )
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    onSave({
      projectId: projectID,
      ticketIds: ticketIDs,
      description: description.trim(),
      tagIds: tagIDs,
      ...(entry.sourceType === 'MANUAL'
        ? { startedAt: new Date(startedAt).toISOString(), endedAt: new Date(endedAt).toISOString() }
        : {}),
    })
    setEditing(false)
  }
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onClose}>
      <section
        className="modal entry-detail-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="entry-detail-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <ModalHeader
          id="entry-detail-title"
          title={editing ? 'Edit time entry' : 'Time entry details'}
          subtitle={`${entry.sourceType === 'TIMER' ? 'Timer entry' : 'Manual entry'} · ${entry.description}`}
          onClose={onClose}
        />
        {editing ? (
          <form className="stack" onSubmit={submit}>
            <label>
              Task description
              <input
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                maxLength={200}
                required
              />
            </label>
            <label>
              Project
              <SingleSelect
                label="Project"
                value={projectID}
                onValueChange={setProjectID}
                options={projects.map((project) => ({ value: project.id, label: project.name }))}
              />
            </label>
            <label>
              Ticket
              <TicketPicker
                organizationID={organizationID}
                value={ticketIDs}
                onChange={setTicketIDs}
              />
            </label>
            {entry.sourceType === 'MANUAL' && (
              <div className="manual-form">
                <label>
                  Start
                  <input
                    type="datetime-local"
                    value={startedAt}
                    onChange={(event) => setStartedAt(event.target.value)}
                    required
                  />
                </label>
                <label>
                  End
                  <input
                    type="datetime-local"
                    value={endedAt}
                    onChange={(event) => setEndedAt(event.target.value)}
                    required
                  />
                </label>
              </div>
            )}
            <fieldset>
              <legend>Tags</legend>
              <TagMultiSelect tags={tags} selectedIds={tagIDs} onToggle={toggle} />
            </fieldset>
            <ModalFooter>
              <button className="secondary" type="button" onClick={() => setEditing(false)}>
                Cancel
              </button>
              <button type="submit">Save changes</button>
            </ModalFooter>
          </form>
        ) : (
          <>
            <dl className="entry-detail-list">
              <div>
                <dt>Project</dt>
                <dd>{entry.projectName}</dd>
              </div>
              <div>
                <dt>Member</dt>
                <dd>{entry.userName}</dd>
              </div>
              <div>
                <dt>Ticket</dt>
                <dd>
                  {entry.tickets.length
                    ? entry.tickets
                        .map((ticket) => `${ticket.reference} · ${ticket.title}`)
                        .join(', ')
                    : 'None'}
                </dd>
              </div>
              <div>
                <dt>Duration</dt>
                <dd>{hoursMinutes(entry.durationSeconds)}</dd>
              </div>
              <div>
                <dt>Start</dt>
                <dd>{new Date(entry.startedAt).toLocaleString()}</dd>
              </div>
              <div>
                <dt>End</dt>
                <dd>{new Date(entry.endedAt).toLocaleString()}</dd>
              </div>
              <div>
                <dt>Tags</dt>
                <dd>{entry.tags.length ? entry.tags.map((tag) => tag.name).join(', ') : 'None'}</dd>
              </div>
            </dl>
            {entry.events.length > 0 && (
              <div className="entry-events">
                <h3>Timer history</h3>
                {entry.events.map((event, index) => (
                  <div key={`${event.type}-${event.occurredAt}-${index}`}>
                    <strong>{event.type}</strong>
                    <span>{new Date(event.occurredAt).toLocaleString()}</span>
                  </div>
                ))}
              </div>
            )}
            {canManage && entry.sourceType === 'TIMER' && (
              <TimerEventEditor events={entry.events} onSave={onSaveEvents} />
            )}
            <ModalFooter>
              <button type="button" className="secondary" onClick={onClose}>
                Close
              </button>
              {canManage && (
                <button type="button" className="secondary" onClick={() => setEditing(true)}>
                  Edit
                </button>
              )}
              {canManage && (
                <button type="button" className="danger-button" onClick={onDelete}>
                  Delete
                </button>
              )}
            </ModalFooter>
          </>
        )}
      </section>
    </div>
  )
}

function TimerEventEditor({
  events,
  onSave,
}: {
  events: CompletedEntryDetail['events']
  onSave: (events: { type: string; occurredAt: string }[]) => void
}) {
  const [drafts, setDrafts] = useState(() =>
    events.map((event) => ({
      type: event.type,
      occurredAt: localDateTimeSecondInput(new Date(event.occurredAt)),
    })),
  )
  function changeTime(index: number, occurredAt: string) {
    setDrafts((values) =>
      values.map((event, eventIndex) => (eventIndex === index ? { ...event, occurredAt } : event)),
    )
  }
  function removePair(index: number) {
    setDrafts((values) =>
      values.filter((_, eventIndex) => eventIndex !== index && eventIndex !== index + 1),
    )
  }
  function addPair() {
    setDrafts((values) => {
      const stopIndex = values.length - 1
      const runningAt = new Date(values[stopIndex - 1].occurredAt).getTime()
      const stoppedAt = new Date(values[stopIndex].occurredAt).getTime()
      if (stoppedAt - runningAt < 3000) return values
      const pauseAt = localDateTimeSecondInput(
        new Date(runningAt + Math.floor((stoppedAt - runningAt) / 3)),
      )
      const resumeAt = localDateTimeSecondInput(
        new Date(runningAt + Math.floor(((stoppedAt - runningAt) * 2) / 3)),
      )
      return [
        ...values.slice(0, stopIndex),
        { type: 'PAUSE' as const, occurredAt: pauseAt },
        { type: 'RESUME' as const, occurredAt: resumeAt },
        values[stopIndex],
      ]
    })
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    onSave(
      drafts.map((item) => ({
        type: item.type,
        occurredAt: new Date(item.occurredAt).toISOString(),
      })),
    )
  }
  return (
    <details className="timer-event-editor">
      <summary>Edit timer history</summary>
      <form className="stack" onSubmit={submit}>
        {drafts.map((event, index) => (
          <div className="timer-event-edit-row" key={`${event.type}-${index}`}>
            <strong>{event.type}</strong>
            <input
              type="datetime-local"
              step="1"
              value={event.occurredAt}
              onChange={(change) => changeTime(index, change.target.value)}
              required
            />
            {event.type === 'PAUSE' ? (
              <IconButton
                label="Remove pause and resume"
                icon="⌫"
                danger
                onClick={() => removePair(index)}
              />
            ) : (
              <span />
            )}
          </div>
        ))}
        <div className="modal-actions">
          <button type="button" className="secondary" onClick={addPair}>
            Add pause pair
          </button>
          <button>Save timer history</button>
        </div>
      </form>
    </details>
  )
}
