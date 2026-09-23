import { useEffect, useRef } from 'react'
import { useTracker } from './useTracker'
import { duration, hoursMinutes, weekDay, displayDate } from '../../utils/time'
import { IconButton } from '../../components/ui/IconButton'
import { ModalHeader } from '../../components/ui/ModalHeader'
import { ModalFooter } from '../../components/ui/ModalFooter'
import { TagMultiSelect } from '../../components/ui/TagMultiSelect'
import { SingleSelect } from '../../components/ui/SingleSelect'
import { EmptyState } from '../../components/ui/EmptyState'
import { EntryDetailDialog } from '../entries/EntryDetailDialog'
import { useEntryDetail } from '../entries/useEntryDetail'
import { TicketPicker } from '../tickets/TicketPicker'

export function TrackerPage() {
  const {
    timer,
    elapsed,
    activeOrganization,
    setTimerFormOpen,
    organizationID,
    timerAction,
    busy,
    timerFormOpen,
    timerProject,
    setTimerProject,
    timerDescription,
    setTimerDescription,
    projects,
    tags,
    timerTags,
    timerTicketIDs,
    setTimerTicketIDs,
    toggleTag,
    saveTimer,
    startTimer,
    todayTotal,
    weekSummary,
    selectedWeekStart,
    selectedWeekEnd,
    currentWeekStart,
    navigateWeek,
    weekTotal,
    todayDate,
    selectedDate,
    setSelectedDate,
    activeTotal,
    entries,
    setManualFormOpen,
    manualFormOpen,
    createManualEntry,
    manualProject,
    setManualProject,
    manualDescription,
    setManualDescription,
    manualStartedAt,
    setManualStartedAt,
    manualMode,
    setManualMode,
    manualEndedAt,
    setManualEndedAt,
    manualDurationMinutes,
    setManualDurationMinutes,
    manualTags,
    manualTicketIDs,
    setManualTicketIDs,
    toggleManualTag,
  } = useTracker()
  const detail = useEntryDetail()
  const manualDialogRef = useRef<HTMLElement>(null)

  useEffect(() => {
    if (!manualFormOpen) return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setManualFormOpen(false)
        return
      }
      if (event.key !== 'Tab') return
      const controls = manualDialogRef.current?.querySelectorAll<HTMLElement>(
        'button:not(:disabled), input:not(:disabled), [tabindex]:not([tabindex="-1"])',
      )
      if (!controls?.length) return
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [manualFormOpen, setManualFormOpen])

  return (
    <div className="tracker-page">
      {timer && (
        <section className="active-timer-bar">
          <div>
            <p className="timer-status">
              <span className={`status-dot ${timer.status === 'RUNNING' ? 'running' : ''}`} />
              {timer.status === 'RUNNING' ? 'Timer running' : 'Timer paused'}
            </p>
            <strong>{duration(elapsed)}</strong>
            <span>{timer.description}</span>
            <small>{activeOrganization?.name ?? 'Active workspace'}</small>
          </div>
          <div className="timer-actions">
            <button
              type="button"
              className="secondary"
              aria-label="Edit timer details"
              title="Edit timer details"
              aria-expanded={timerFormOpen}
              onClick={() => setTimerFormOpen((value) => !value)}
              disabled={timer.organizationId !== organizationID}
            >
              <TimerControlIcon name="edit" />
            </button>
            {timer.status === 'RUNNING' ? (
              <button
                type="button"
                aria-label="Pause timer"
                title="Pause timer"
                onClick={() => void timerAction('pause')}
                disabled={busy}
              >
                <TimerControlIcon name="pause" />
              </button>
            ) : (
              <button
                type="button"
                aria-label="Resume timer"
                title="Resume timer"
                onClick={() => void timerAction('resume')}
                disabled={busy}
              >
                <TimerControlIcon name="play" />
              </button>
            )}
            <button
              type="button"
              className="secondary"
              aria-label="Stop timer"
              title="Stop timer"
              onClick={() => void timerAction('stop')}
              disabled={busy}
            >
              <TimerControlIcon name="stop" />
            </button>
          </div>
        </section>
      )}
      {timerFormOpen && (
        <section className="card tracker-form-card">
          <div className="section-heading">
            <div>
              <h2>{timer ? 'Timer details' : 'Start timer'}</h2>
              <p>
                {timer
                  ? 'Update the task, project, or tags for the active timer.'
                  : 'Choose where this time should be recorded.'}
              </p>
            </div>
            <IconButton label="Close timer form" icon="×" onClick={() => setTimerFormOpen(false)} />
          </div>
          {timer && timer.organizationId !== organizationID ? (
            <p className="muted">
              Select {activeOrganization?.name ?? 'the active workspace'} to update this timer.
            </p>
          ) : (
            <div className="timer-form-row">
              <label>
                Project
                <SingleSelect
                  label="Project"
                  value={timerProject}
                  onValueChange={setTimerProject}
                  disabled={busy}
                  options={[
                    { value: '', label: 'Select project' },
                    ...projects.map((project) => ({ value: project.id, label: project.name })),
                  ]}
                />
              </label>
              <fieldset className="timer-tag-field">
                <legend>Tags</legend>
                <TagMultiSelect
                  tags={tags}
                  selectedIds={timerTags}
                  onToggle={toggleTag}
                  showRecentByDefault
                  disabled={busy}
                />
              </fieldset>
              <label>
                Ticket
                <TicketPicker
                  organizationID={organizationID}
                  value={timerTicketIDs}
                  onChange={setTimerTicketIDs}
                  disabled={busy}
                />
              </label>
              <label>
                Task description
                <input
                  value={timerDescription}
                  onChange={(event) => setTimerDescription(event.target.value)}
                  maxLength={200}
                  placeholder="Investigating the bug"
                  required
                  disabled={busy}
                />
              </label>
              {timer ? (
                <button
                  type="button"
                  className="secondary"
                  onClick={() => void saveTimer()}
                  disabled={busy || !timerProject || !timerDescription.trim()}
                >
                  Save changes
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => void startTimer()}
                  disabled={busy || !timerProject || !timerDescription.trim()}
                >
                  Start
                </button>
              )}
            </div>
          )}
        </section>
      )}
      <section className="summary-grid">
        <article className="summary-card today-summary">
          <span>Today total{timer?.status === 'RUNNING' ? ' · live' : ''}</span>
          <strong>{hoursMinutes(todayTotal)}</strong>
          {!timer && (
            <button
              type="button"
              className="start-timer-trigger"
              aria-label="Start timer"
              title="Start timer"
              aria-expanded={timerFormOpen}
              onClick={() => {
                setTimerFormOpen((value) => !value)
                setManualFormOpen(false)
              }}
            >
              <span aria-hidden="true">▶</span>
            </button>
          )}
        </article>
        <article className="summary-card week-summary">
          <div className="week-summary-header">
            <div className="week-total-label">
              <span>Week total</span>
              <strong>{weekSummary ? hoursMinutes(weekTotal) : '…'}</strong>
            </div>
            <div className="week-navigation" aria-label="Select week">
              <button
                type="button"
                aria-label="Previous week"
                title="Previous week"
                onClick={() => navigateWeek(-1)}
              >
                &lt;
              </button>
              <span className="week-range">
                {displayDate(selectedWeekStart)} - {displayDate(selectedWeekEnd)}
              </span>
              <button
                type="button"
                aria-label="Next week"
                title="Next week"
                onClick={() => navigateWeek(1)}
              >
                &gt;
              </button>
            </div>
          </div>
          <div className="week-breakdown">
            {(weekSummary?.days ?? []).map((day) => (
              <button
                type="button"
                className={`week-day ${day.date === todayDate ? 'today' : ''} ${day.date === selectedDate ? 'selected' : ''}`}
                key={day.date}
                onClick={() => setSelectedDate(day.date)}
              >
                <span>{weekDay(day.date)}</span>
                <small>{day.date.slice(-2)}</small>
                <strong>
                  {hoursMinutes(
                    day.durationSeconds +
                      (selectedWeekStart === currentWeekStart && day.date === todayDate
                        ? activeTotal
                        : 0),
                  )}
                </strong>
              </button>
            ))}
          </div>
        </article>
      </section>
      <section className="card today-entries">
        <div className="section-heading">
          <div>
            <h2>{selectedDate === todayDate ? 'Today' : displayDate(selectedDate)}</h2>
            <p>Completed time entries for the selected day.</p>
          </div>
          <span className="count-chip">{entries.length}</span>
        </div>
        {entries.length ? (
          <div className="entry-list">
            {entries.map((entry) => (
              <div className="entry-row" key={entry.id}>
                <div>
                  <strong>{entry.description}</strong>
                  <small>
                    {entry.projectName}
                    {entry.ticketReferences.length ? ` · ${entry.ticketReferences.join(', ')}` : ''}
                  </small>
                  <small>
                    {new Date(entry.startedAt).toLocaleTimeString([], {
                      hour: '2-digit',
                      minute: '2-digit',
                    })}{' '}
                    –{' '}
                    {new Date(entry.endedAt).toLocaleTimeString([], {
                      hour: '2-digit',
                      minute: '2-digit',
                    })}
                  </small>
                  {entry.tags.length > 0 && (
                    <span className="entry-tags">
                      {entry.tags.map((tag) => (
                        <em key={tag}>{tag}</em>
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
            ))}
          </div>
        ) : (
          <EmptyState
            title="No completed entries"
            text="Completed work for the selected day will appear here."
            compact
          />
        )}
      </section>
      <div className="tracker-create-actions">
        <button
          className="secondary"
          onClick={() => {
            setManualFormOpen(true)
            setTimerFormOpen(false)
          }}
        >
          Add manual entry
        </button>
      </div>
      {manualFormOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
          onMouseDown={() => setManualFormOpen(false)}
        >
          <section
            className="modal manual-entry-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="manual-entry-title"
            ref={manualDialogRef}
            onMouseDown={(event) => event.stopPropagation()}
          >
            <ModalHeader
              id="manual-entry-title"
              title="Add manual entry"
              subtitle="Record past or current work in minute-level precision."
              onClose={() => setManualFormOpen(false)}
            />
            <form className="manual-form" onSubmit={(event) => void createManualEntry(event)}>
              <label>
                Task description
                <input
                  value={manualDescription}
                  onChange={(event) => setManualDescription(event.target.value)}
                  maxLength={200}
                  placeholder="Investigating the bug"
                  required
                  autoFocus
                />
              </label>
              <label>
                Project
                <SingleSelect
                  label="Project"
                  value={manualProject}
                  onValueChange={setManualProject}
                  required
                  options={projects.map((project) => ({ value: project.id, label: project.name }))}
                />
              </label>
              <label>
                Ticket
                <TicketPicker
                  organizationID={organizationID}
                  value={manualTicketIDs}
                  onChange={setManualTicketIDs}
                  disabled={busy}
                />
              </label>
              <label>
                Start
                <input
                  type="datetime-local"
                  value={manualStartedAt}
                  onChange={(event) => setManualStartedAt(event.target.value)}
                  required
                />
              </label>
              <div className="manual-mode">
                <button
                  type="button"
                  className={manualMode === 'end' ? '' : 'secondary'}
                  onClick={() => setManualMode('end')}
                >
                  End time
                </button>
                <button
                  type="button"
                  className={manualMode === 'duration' ? '' : 'secondary'}
                  onClick={() => setManualMode('duration')}
                >
                  Duration
                </button>
              </div>
              {manualMode === 'end' ? (
                <label>
                  End
                  <input
                    type="datetime-local"
                    value={manualEndedAt}
                    onChange={(event) => setManualEndedAt(event.target.value)}
                    required
                  />
                </label>
              ) : (
                <label>
                  Duration (minutes)
                  <input
                    type="number"
                    min="1"
                    step="1"
                    value={manualDurationMinutes}
                    onChange={(event) => setManualDurationMinutes(event.target.value)}
                    required
                  />
                </label>
              )}
              <fieldset>
                <legend>Tags</legend>
                <TagMultiSelect
                  tags={tags}
                  selectedIds={manualTags}
                  onToggle={toggleManualTag}
                  showRecentByDefault
                />
              </fieldset>
              <ModalFooter>
                <button
                  type="button"
                  className="secondary"
                  onClick={() => setManualFormOpen(false)}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={busy || !manualProject || !manualDescription.trim()}
                >
                  Add entry
                </button>
              </ModalFooter>
            </form>
          </section>
        </div>
      )}
      <EntryDetailDialog detail={detail} />
    </div>
  )
}

function TimerControlIcon({ name }: { name: 'edit' | 'pause' | 'play' | 'stop' }) {
  return (
    <svg
      aria-hidden="true"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {name === 'edit' && (
        <>
          <path d="M12 20h9" />
          <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L9 17l-4 1 1-4 10.5-10.5Z" />
        </>
      )}
      {name === 'pause' && (
        <>
          <path d="M8 5v14" />
          <path d="M16 5v14" />
        </>
      )}
      {name === 'play' && <path d="m8 5 11 7-11 7V5Z" />}
      {name === 'stop' && <rect x="6" y="6" width="12" height="12" rx="1" />}
    </svg>
  )
}
