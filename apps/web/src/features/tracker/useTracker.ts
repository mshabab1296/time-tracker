import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import type { ActiveTimer, CompletedEntry, WeekSummary } from '../../types/domain'
import { dateInTimezone, duration, localDateTimeInput } from '../../utils/time'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useTracker() {
  const {
    organizationID,
    organizations,
    user,
    projects,
    tags,
    setMessage,
    entriesRevision,
    notifyEntriesChanged,
  } = useWorkspace()
  const [entries, setEntries] = useState<CompletedEntry[]>([])

  const [weekSummary, setWeekSummary] = useState<WeekSummary | null>(null)

  const [currentWeekSummary, setCurrentWeekSummary] = useState<WeekSummary | null>(null)

  const [selectedWeekStart, setSelectedWeekStart] = useState(() =>
    mondayOfWeek(dateInTimezone(user.timezone)),
  )

  const weekRequestRef = useRef(0)

  const currentWeekCacheRef = useRef<{
    organizationID: string
    weekStart: string
    entriesRevision: number
    summary: WeekSummary
  } | null>(null)

  const [timer, setTimer] = useState<ActiveTimer | null>(null)

  const [timerProject, setTimerProject] = useState('')

  const [timerDescription, setTimerDescription] = useState('')

  const [timerTags, setTimerTags] = useState<string[]>([])
  const [timerTicketIDs, setTimerTicketIDs] = useState<string[]>([])

  const [manualProject, setManualProject] = useState('')

  const [manualDescription, setManualDescription] = useState('')

  const [manualTags, setManualTags] = useState<string[]>([])
  const [manualTicketIDs, setManualTicketIDs] = useState<string[]>([])

  const [manualStartedAt, setManualStartedAt] = useState(() =>
    localDateTimeInput(new Date(Date.now() - 60 * 60 * 1000)),
  )

  const [manualEndedAt, setManualEndedAt] = useState(() => localDateTimeInput())

  const [manualMode, setManualMode] = useState<'end' | 'duration'>('end')

  const [manualDurationMinutes, setManualDurationMinutes] = useState('60')

  const [timerAt, setTimerAt] = useState(Date.now())

  const [clock, setClock] = useState(Date.now())

  const [busy, setBusy] = useState(false)

  const [selectedDate, setSelectedDate] = useState(() => dateInTimezone(user.timezone))

  const [timerFormOpen, setTimerFormOpen] = useState(false)

  const [manualFormOpen, setManualFormOpen] = useState(false)

  const activeOrganization = useMemo(
    () => organizations.find((item) => item.id === timer?.organizationId),
    [organizations, timer],
  )

  const elapsed = timer
    ? timer.durationSeconds +
      (timer.status === 'RUNNING' ? Math.max(0, Math.floor((clock - timerAt) / 1000)) : 0)
    : 0

  const todayDate = dateInTimezone(user.timezone)

  const currentWeekStart = mondayOfWeek(todayDate)

  const stoppedTodayTotal =
    currentWeekSummary?.days.find((day) => day.date === todayDate)?.durationSeconds ??
    (selectedDate === todayDate
      ? entries.reduce((total, entry) => total + entry.durationSeconds, 0)
      : 0)

  const completedWeekTotal =
    weekSummary?.days.reduce((total, day) => total + day.durationSeconds, 0) ?? 0

  const activeTotal = timer ? elapsed : 0

  const todayTotal = stoppedTodayTotal + activeTotal

  const weekTotal = completedWeekTotal + (selectedWeekStart === currentWeekStart ? activeTotal : 0)

  function navigateWeek(weeks: number) {
    const nextWeekStart = shiftDate(selectedWeekStart, weeks * 7)
    setSelectedWeekStart(nextWeekStart)
    setSelectedDate(nextWeekStart)
    setWeekSummary(null)
  }

  const receiveTimer = useCallback((value: ActiveTimer | null) => {
    setTimer(value)
    setTimerAt(Date.now())
    if (value) {
      setTimerProject(value.projectId)
      setTimerDescription(value.description)
      setTimerTags(value.tagIds)
      setTimerTicketIDs(value.ticketIds ?? [])
    }
  }, [])

  const loadTimer = useCallback(async () => {
    if (!organizationID) {
      receiveTimer(null)
      return
    }
    const result = await api<ActiveTimer | null>(
      `/timer/active?organizationId=${encodeURIComponent(organizationID)}`,
    )
    if (!result.error) receiveTimer(result.data)
  }, [organizationID, receiveTimer])

  const loadTodayEntries = useCallback(async () => {
    if (!organizationID) {
      setEntries([])
      return
    }
    const result = await api<CompletedEntry[]>(
      `/time-entries/today?organizationId=${encodeURIComponent(organizationID)}&date=${encodeURIComponent(selectedDate)}`,
    )
    if (!result.error) setEntries(result.data)
  }, [organizationID, selectedDate])

  const loadWeekSummary = useCallback(async () => {
    const requestID = ++weekRequestRef.current
    if (!organizationID) {
      setWeekSummary(null)
      setCurrentWeekSummary(null)
      return
    }
    const base = `/time-entries/week-summary?organizationId=${encodeURIComponent(organizationID)}`
    const cache = currentWeekCacheRef.current
    const cachedCurrent =
      cache?.organizationID === organizationID &&
      cache.weekStart === currentWeekStart &&
      cache.entriesRevision === entriesRevision
        ? cache.summary
        : null
    const current = cachedCurrent
      ? { data: cachedCurrent, error: null }
      : await api<WeekSummary>(`${base}&date=${encodeURIComponent(currentWeekStart)}`)
    if (requestID !== weekRequestRef.current) return
    if (!current.error) {
      currentWeekCacheRef.current = {
        organizationID,
        weekStart: currentWeekStart,
        entriesRevision,
        summary: current.data,
      }
    }
    const selected =
      selectedWeekStart === currentWeekStart
        ? current
        : await api<WeekSummary>(`${base}&date=${encodeURIComponent(selectedWeekStart)}`)
    if (requestID !== weekRequestRef.current) return
    if (!current.error) setCurrentWeekSummary(current.data)
    if (!selected.error) setWeekSummary(selected.data)
    if (current.error || selected.error)
      setMessage(
        (current.error ?? selected.error)?.message ?? 'Unable to load weekly totals.',
        'error',
      )
  }, [organizationID, currentWeekStart, selectedWeekStart, entriesRevision, setMessage])

  function toggleTag(id: string) {
    setTimerTags((values) =>
      values.includes(id) ? values.filter((value) => value !== id) : [...values, id],
    )
  }

  function toggleManualTag(id: string) {
    setManualTags((values) =>
      values.includes(id) ? values.filter((value) => value !== id) : [...values, id],
    )
  }

  async function startTimer() {
    if (!organizationID || !timerProject || !timerDescription.trim()) return
    setBusy(true)
    const result = await api<ActiveTimer>('/timer/start', {
      method: 'POST',
      body: JSON.stringify({
        organizationId: organizationID,
        projectId: timerProject,
        description: timerDescription.trim(),
        tagIds: timerTags,
        ticketIds: timerTicketIDs,
      }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      receiveTimer(result.data)
      setTimerFormOpen(false)
      setMessage('Timer started.')
    }
    setBusy(false)
  }

  async function createManualEntry(event: FormEvent) {
    event.preventDefault()
    if (!organizationID || !manualProject || !manualDescription.trim()) return
    const startedAt = new Date(manualStartedAt)
    const body =
      manualMode === 'end'
        ? {
            organizationId: organizationID,
            projectId: manualProject,
            description: manualDescription.trim(),
            tagIds: manualTags,
            ticketIds: manualTicketIDs,
            startedAt: startedAt.toISOString(),
            endedAt: new Date(manualEndedAt).toISOString(),
          }
        : {
            organizationId: organizationID,
            projectId: manualProject,
            description: manualDescription.trim(),
            tagIds: manualTags,
            ticketIds: manualTicketIDs,
            startedAt: startedAt.toISOString(),
            durationMinutes: Number(manualDurationMinutes),
          }
    setBusy(true)
    const result = await api<CompletedEntry>('/time-entries/manual', {
      method: 'POST',
      body: JSON.stringify(body),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setManualFormOpen(false)
      setManualDescription('')
      notifyEntriesChanged()
      setMessage('Manual entry added.')
    }
    setBusy(false)
  }

  async function timerAction(action: 'pause' | 'resume' | 'stop') {
    if (!timer) return
    setBusy(true)
    const result = await api<ActiveTimer>(`/timer/${action}`, {
      method: 'POST',
      body: JSON.stringify({ entryId: timer.id }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else if (action === 'stop') {
      receiveTimer(null)
      notifyEntriesChanged()
      setMessage(`Timer stopped after ${duration(result.data.durationSeconds)}.`)
    } else receiveTimer(result.data)
    setBusy(false)
  }

  async function saveTimer() {
    if (!timer || timer.organizationId !== organizationID || !timerDescription.trim()) return
    setBusy(true)
    const result = await api<ActiveTimer>('/timer/active', {
      method: 'PATCH',
      body: JSON.stringify({
        entryId: timer.id,
        projectId: timerProject,
        description: timerDescription.trim(),
        tagIds: timerTags,
        ticketIds: timerTicketIDs,
      }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      receiveTimer(result.data)
      setMessage('Timer details saved.')
    }
    setBusy(false)
  }
  useEffect(() => {
    void loadTimer()
  }, [loadTimer])
  useEffect(() => {
    void loadTodayEntries()
  }, [loadTodayEntries, entriesRevision])
  useEffect(() => {
    void loadWeekSummary()
  }, [loadWeekSummary, entriesRevision])
  useEffect(() => {
    if (!timerProject && projects[0]) setTimerProject(projects[0].id)
    if (!manualProject && projects[0]) setManualProject(projects[0].id)
  }, [projects, timerProject, manualProject])
  useEffect(() => {
    const id = window.setInterval(() => setClock(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [])
  return {
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
    selectedWeekEnd: shiftDate(selectedWeekStart, 6),
    navigateWeek,
    currentWeekStart,
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
  }
}

function shiftDate(date: string, days: number) {
  const [year, month, day] = date.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1, day + days, 12)).toISOString().slice(0, 10)
}

function mondayOfWeek(date: string) {
  const weekday = new Date(`${date}T12:00:00Z`).getUTCDay()
  return shiftDate(date, -((weekday + 6) % 7))
}
