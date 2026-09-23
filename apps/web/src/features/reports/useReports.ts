import { useState } from 'react'
import { api, type APIResponse } from '../../api/client'
import type { ReportPage, SummaryPage, SummaryValue } from '../../types/domain'
import { firstDayOfMonth, localDate } from '../../utils/time'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useReports() {
  const { organizationID, isAdmin, members, projects, tags, setMessage } = useWorkspace()
  const [report, setReport] = useState<ReportPage | null>(null)

  const [summary, setSummary] = useState<SummaryPage | null>(null)

  const [reportView, setReportView] = useState<'detailed' | 'summary'>('detailed')

  const [reportScope, setReportScope] = useState<'personal' | 'organization'>('personal')

  const [reportStartDate, setReportStartDate] = useState(firstDayOfMonth)

  const [reportEndDate, setReportEndDate] = useState(() => localDate())

  const [reportUserID, setReportUserID] = useState('')

  const [reportProjectID, setReportProjectID] = useState('')
  const [reportTicketIDs, setReportTicketIDs] = useState<string[]>([])

  const [reportTagIDs, setReportTagIDs] = useState<string[]>([])

  const [reportGroupBy, setReportGroupBy] = useState<SummaryValue['dimension'][]>([
    'project',
    'date',
  ])

  const [reportDateGrouping, setReportDateGrouping] = useState<'day' | 'week' | 'month'>('day')

  const [reportOffset, setReportOffset] = useState(0)

  const [reportLoading, setReportLoading] = useState(false)

  function reportParameters(offset = 0) {
    const parameters = new URLSearchParams({
      organizationId: organizationID,
      scope: reportScope,
      startDate: reportStartDate,
      endDate: reportEndDate,
      limit: '25',
      offset: String(offset),
      dateGrouping: reportDateGrouping,
    })
    if (reportScope === 'organization' && reportUserID) parameters.set('userId', reportUserID)
    if (reportProjectID) parameters.set('projectId', reportProjectID)
    reportTicketIDs.forEach((id) => parameters.append('ticketId', id))
    reportTagIDs.forEach((id) => parameters.append('tagId', id))
    reportGroupBy.forEach((dimension) => parameters.append('groupBy', dimension))
    return parameters
  }

  async function loadReport(offset = reportOffset) {
    if (!organizationID) return
    setReportLoading(true)
    try {
      if (reportView === 'detailed') {
        const result = await api<ReportPage>(`/reports/entries?${reportParameters(offset)}`)
        if (result.error) setMessage(result.error.message, 'error')
        else {
          setReport(result.data)
          setReportOffset(offset)
        }
      } else {
        const result = await api<SummaryPage>(`/reports/summary?${reportParameters(offset)}`)
        if (result.error) setMessage(result.error.message, 'error')
        else {
          setSummary(result.data)
          setReportOffset(offset)
        }
      }
    } finally {
      setReportLoading(false)
    }
  }

  async function downloadReport(format: 'detailed' | 'summary') {
    if (!organizationID) return
    setReportLoading(true)
    try {
      const response = await fetch(
        `/api/v1/reports/export?format=${format}&${reportParameters(0)}`,
        { credentials: 'include' },
      )
      if (!response.ok) {
        const result = (await response.json()) as APIResponse<never>
        setMessage(result.error?.message ?? 'Unable to download the report.', 'error')
        return
      }
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download =
        response.headers.get('Content-Disposition')?.match(/filename="([^"]+)"/)?.[1] ??
        `time-report-${format}.csv`
      link.click()
      URL.revokeObjectURL(url)
    } finally {
      setReportLoading(false)
    }
  }

  function toggleReportTag(id: string) {
    setReportTagIDs((values) =>
      values.includes(id) ? values.filter((value) => value !== id) : [...values, id],
    )
  }

  function toggleReportGroup(dimension: SummaryValue['dimension']) {
    setReportGroupBy((values) =>
      values.includes(dimension)
        ? values.filter((value) => value !== dimension)
        : [...values, dimension],
    )
  }

  function moveReportGroup(index: number, direction: -1 | 1) {
    setReportGroupBy((values) => {
      const target = index + direction
      if (target < 0 || target >= values.length) return values
      const result = [...values]
      ;[result[index], result[target]] = [result[target], result[index]]
      return result
    })
  }

  return {
    reportView,
    report,
    summary,
    reportScope,
    setReportView,
    setReportOffset,
    reportStartDate,
    setReportStartDate,
    reportEndDate,
    setReportEndDate,
    isAdmin,
    setReportScope,
    setReportUserID,
    setReportGroupBy,
    reportUserID,
    members,
    reportProjectID,
    organizationID,
    reportTicketIDs,
    setReportTicketIDs,
    setReportProjectID,
    projects,
    tags,
    reportTagIDs,
    toggleReportTag,
    reportGroupBy,
    toggleReportGroup,
    moveReportGroup,
    reportDateGrouping,
    setReportDateGrouping,
    reportLoading,
    loadReport,
    downloadReport,
  }
}
