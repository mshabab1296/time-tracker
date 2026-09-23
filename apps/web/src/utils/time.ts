import type { WeekSummary } from '../types/domain'

export function duration(seconds: number) {
  return `${String(Math.floor(seconds / 3600)).padStart(2, '0')}:${String(Math.floor((seconds % 3600) / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`
}
export function hoursMinutes(seconds: number) {
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}
export function dateInTimezone(timezone: string) {
  const values = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
    .formatToParts(new Date())
    .reduce<Record<string, string>>((result, part) => ({ ...result, [part.type]: part.value }), {})
  return `${values.year}-${values.month}-${values.day}`
}
export function weekDay(date: string) {
  const [year, month, day] = date.split('-').map(Number)
  return new Intl.DateTimeFormat('en-US', { timeZone: 'UTC', weekday: 'short' }).format(
    new Date(Date.UTC(year, month - 1, day, 12)),
  )
}
export function localDateTimeInput(date = new Date()) {
  const offsetDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return offsetDate.toISOString().slice(0, 16)
}
export function localDateTimeSecondInput(date: Date) {
  const offsetDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return offsetDate.toISOString().slice(0, 19)
}
export function localDate(date = new Date()) {
  const offsetDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return offsetDate.toISOString().slice(0, 10)
}
export function firstDayOfMonth() {
  const now = new Date()
  return localDate(new Date(now.getFullYear(), now.getMonth(), 1))
}
export function reportDateTime(value: string, timezone: string) {
  return new Date(value).toLocaleString([], { timeZone: timezone })
}
export function displayDate(date: string) {
  const [year, month, day] = date.split('-').map(Number)
  return new Intl.DateTimeFormat('en-US', { timeZone: 'UTC', month: 'short', day: 'numeric' })
    .format(new Date(Date.UTC(year, month - 1, day, 12)))
    .replace(/^Sep /, 'Sept ')
}
export function displayWeekRange(days: WeekSummary['days']) {
  return days.length
    ? `${displayDate(days[0].date)} - ${displayDate(days[days.length - 1].date)}`
    : 'Current week'
}
