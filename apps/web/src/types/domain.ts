export type User = {
  id: string
  email: string
  name: string
  timezone: string
  emailVerified: boolean
}
export type Organization = { id: string; name: string; timezone: string; role: 'ADMIN' | 'MEMBER' }
export type Project = { id: string; name: string; assignedMemberIds?: string[] }
export type Tag = { id: string; name: string; createdAt?: string }
export type Ticket = {
  id: string
  organizationId: string
  createdByUserId: string
  reference: string
  title: string
  createdAt: string
}
export type TicketPage = { items: Ticket[]; total: number; limit: number; offset: number }
export type Member = { id: string; email: string; name: string; role: 'ADMIN' | 'MEMBER' }
export type Invitation = { id: string; organizationName: string; expiresAt: string }
export type SentInvitation = {
  id: string
  email: string
  status: 'PENDING' | 'ACCEPTED' | 'DECLINED' | 'EXPIRED' | 'CANCELLED'
  expiresAt: string
}
export type ActiveTimer = {
  id: string
  organizationId: string
  projectId: string
  ticketIds: string[]
  description: string
  tagIds: string[]
  status: 'RUNNING' | 'PAUSED'
  durationSeconds: number
}
export type CompletedEntry = {
  id: string
  projectName: string
  ticketReferences: string[]
  description: string
  startedAt: string
  endedAt: string
  durationSeconds: number
  sourceType: 'TIMER' | 'MANUAL'
  tags: string[]
}
export type EntryTag = { id: string; name: string }
export type EntryTicket = { id: string; reference: string; title: string }
export type CompletedEntryItem = {
  id: string
  organizationId: string
  userId: string
  userName: string
  projectId: string
  projectName: string
  tickets: EntryTicket[]
  description: string
  sourceType: 'TIMER' | 'MANUAL'
  startedAt: string
  endedAt: string
  durationSeconds: number
  tags: EntryTag[]
}
export type CompletedEntryPage = {
  items: CompletedEntryItem[]
  total: number
  limit: number
  offset: number
}
export type CompletedEntryDetail = CompletedEntryItem & {
  events: { type: 'START' | 'PAUSE' | 'RESUME' | 'STOP'; occurredAt: string }[]
}
export type ReportEntryItem = CompletedEntryItem & { reportDurationSeconds: number }
export type ReportPage = {
  items: ReportEntryItem[]
  total: number
  totalDurationSeconds: number
  limit: number
  offset: number
  timezone: string
  scope: 'personal' | 'organization'
  startDate: string
  endDate: string
}
export type SummaryValue = {
  dimension: 'member' | 'project' | 'tag' | 'ticket' | 'date'
  key: string
  label: string
}
export type SummaryPage = {
  items: { values: SummaryValue[]; durationSeconds: number }[]
  total: number
  totalDurationSeconds: number
  limit: number
  offset: number
  timezone: string
  scope: 'personal' | 'organization'
  startDate: string
  endDate: string
  groupBy: SummaryValue['dimension'][]
  dateGrouping: 'day' | 'week' | 'month'
}
export type WeekSummary = { weekStart: string; days: { date: string; durationSeconds: number }[] }
export type Page =
  'tracker' | 'entries' | 'reports' | 'projects' | 'tickets' | 'tags' | 'team' | 'settings'
export type RenameTarget = { kind: 'projects' | 'tags'; resource: Project | Tag }
