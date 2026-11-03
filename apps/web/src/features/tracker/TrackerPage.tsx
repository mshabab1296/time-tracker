import { useTracker } from './useTracker'
import { dateInTimezone, duration, hoursMinutes, weekDay, displayDate } from '../../utils/time'
import { IconButton } from '../../components/ui/IconButton'
import { ModalHeader } from '../../components/ui/ModalHeader'
import { Modal } from '../../components/ui/Modal'
import { ModalFooter } from '../../components/ui/ModalFooter'
import { TagMultiSelect } from '../../components/ui/TagMultiSelect'
import { SingleSelect } from '../../components/ui/SingleSelect'
import { EmptyState } from '../../components/ui/EmptyState'
import { EntryDetailDialog } from '../entries/EntryDetailDialog'
import { useEntryDetail } from '../entries/useEntryDetail'
import { TicketPicker } from '../tickets/TicketPicker'

export function TrackerPage() {
  const {
    readOnly,
    selectedTimezone,
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
  const activeProjectName = projects.find((project) => project.id === timer?.projectId)?.name

  return (
    <div className="tracker-page">
      {timer && (
        <section className="active-timer-bar">
          <div>
            <p className="timer-status">
              <span className={`status-dot ${timer.status === 'RUNNING' ? 'running' : ''}`} />
            </p>
            <strong>{duration(elapsed)}</strong>
            <span>
              {timer.ticketReferences?.length ? `${timer.ticketReferences.join(', ')}: ` : ''}
              {timer.description}
            </span>
            <small>
              {activeOrganization?.name ?? 'Active workspace'}
              {activeProjectName && ` · ${activeProjectName}`}
            </small>
          </div>
          <div className="timer-actions">
            <button
              type="button"
              className="secondary"
              aria-label={readOnly ? 'View timer details' : 'Edit timer details'}
              title={readOnly ? 'View timer details' : 'Edit timer details'}
              aria-expanded={timerFormOpen}
              onClick={() => setTimerFormOpen((value) => !value)}
              disabled={timer.organizationId !== organizationID}
            >
              <TimerControlIcon name={readOnly ? 'view' : 'edit'} />
            </button>
            {!readOnly &&
              (timer.status === 'RUNNING' ? (
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
              ))}
            {!readOnly && (
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
            )}
          </div>
        </section>
      )}
      {timerFormOpen && (!readOnly || timer) && (
        <section className="card tracker-form-card">
          <div className="section-heading">
            <div>
              <h2>{timer ? 'Timer details' : 'Start timer'}</h2>
              <p>
                {readOnly
                  ? 'View this member’s active timer. Only the member can make changes.'
                  : timer
                    ? 'Update the task, project, or tags for the active timer.'
                    : 'Choose where this time should be recorded.'}
              </p>
            </div>
            <IconButton label="Close timer form" icon="×" onClick={() => setTimerFormOpen(false)} />
          </div>
          {readOnly && timer ? (
            <div className="timer-form-row">
              <div className="timer-readonly-field">
                <span>Project</span>
                <div className="timer-readonly-value">
                  {activeProjectName ?? 'Project unavailable'}
                </div>
              </div>
              <div className="timer-readonly-field">
                <span>Ticket</span>
                <div className="timer-readonly-value">
                  {timer.ticketReferences?.join(', ') || 'None'}
                </div>
              </div>
              <div className="timer-readonly-field">
                <span>Task description</span>
                <div className="timer-readonly-value">{timer.description}</div>
              </div>
              <div className="timer-readonly-field">
                <span>Tags</span>
                <div className="timer-readonly-value">
                  {timer.tagIds
                    .map((id) => tags.find((tag) => tag.id === id)?.name)
                    .filter(Boolean)
                    .join(', ') || 'None'}
                </div>
              </div>
            </div>
          ) : timer && timer.organizationId !== organizationID ? (
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
          {!readOnly && !timer && (
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
                    {clockTime(new Date(entry.dayStartedAt), selectedTimezone)} –{' '}
                    {dayEndClockTime(entry.dayEndedAt, selectedDate, selectedTimezone)}
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
                  {!readOnly && (
                    <IconButton
                      label={`Edit ${entry.description}`}
                      icon="✎"
                      onClick={() => void detail.openEntryDetail(entry.id, 'edit')}
                    />
                  )}
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
      {!readOnly && (
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
      )}
      {!readOnly && manualFormOpen && (
        <Modal
          className="manual-entry-modal"
          titleId="manual-entry-title"
          onClose={() => setManualFormOpen(false)}
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
              <button type="button" className="secondary" onClick={() => setManualFormOpen(false)}>
                Cancel
              </button>
              <button type="submit" disabled={busy || !manualProject || !manualDescription.trim()}>
                Add entry
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}
      <EntryDetailDialog detail={detail} />
    </div>
  )
}

function clockTime(value: Date, timezone: string) {
  return value.toLocaleTimeString('en-US', {
    timeZone: timezone,
    hour: '2-digit',
    minute: '2-digit',
  })
}

function dayEndClockTime(value: string, selectedDate: string, timezone: string) {
  const end = new Date(value)
  // A midnight boundary is exclusive; display the final minute of the selected day.
  const displayEnd =
    dateInTimezone(timezone, end) === selectedDate ? end : new Date(end.getTime() - 1)
  return clockTime(displayEnd, timezone)
}

function TimerControlIcon({ name }: { name: 'view' | 'edit' | 'pause' | 'play' | 'stop' }) {
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
      {name === 'view' && (
        <>
          <path d="M2 12s3.6-6 10-6 10 6 10 6-3.6 6-10 6S2 12 2 12Z" />
          <circle cx="12" cy="12" r="2.5" />
        </>
      )}
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
