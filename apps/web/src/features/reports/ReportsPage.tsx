import { useReports } from './useReports'
import { hoursMinutes, reportDateTime } from '../../utils/time'
import { EmptyState } from '../../components/ui/EmptyState'
import type { SummaryValue } from '../../types/domain'
import { TicketPicker } from '../tickets/TicketPicker'
import { TagMultiSelect } from '../../components/ui/TagMultiSelect'
import { SingleSelect } from '../../components/ui/SingleSelect'

export function ReportsPage() {
  const {
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
  } = useReports()
  const activeReport = reportView === 'detailed' ? report : summary
  const groupOptions: SummaryValue['dimension'][] =
    reportScope === 'organization'
      ? ['member', 'project', 'ticket', 'tag', 'date']
      : ['project', 'ticket', 'tag', 'date']
  return (
    <div className="report-layout">
      <section className="card report-filters">
        <div className="section-heading">
          <div>
            <h2>Report filters</h2>
            <p>Dates are interpreted in the report timezone.</p>
          </div>
          <div className="view-switch">
            <button
              className={reportView === 'detailed' ? '' : 'secondary'}
              onClick={() => {
                setReportView('detailed')
                setReportOffset(0)
              }}
            >
              Detailed
            </button>
            <button
              className={reportView === 'summary' ? '' : 'secondary'}
              onClick={() => {
                setReportView('summary')
                setReportOffset(0)
              }}
            >
              Summary
            </button>
          </div>
        </div>
        <div className="report-filter-grid">
          <label>
            Start date
            <input
              type="date"
              value={reportStartDate}
              onChange={(event) => setReportStartDate(event.target.value)}
            />
          </label>
          <label>
            End date
            <input
              type="date"
              value={reportEndDate}
              onChange={(event) => setReportEndDate(event.target.value)}
            />
          </label>
          {isAdmin && (
            <label>
              Scope
              <SingleSelect
                label="Scope"
                value={reportScope}
                onValueChange={(value) => {
                  const scope = value as 'personal' | 'organization'
                  setReportScope(scope)
                  setReportUserID('')
                  if (scope === 'personal')
                    setReportGroupBy((values) => values.filter((value) => value !== 'member'))
                }}
                options={[
                  { value: 'personal', label: 'My time' },
                  { value: 'organization', label: 'Organization' },
                ]}
              />
            </label>
          )}
          {isAdmin && reportScope === 'organization' && (
            <label>
              Member
              <SingleSelect
                label="Member"
                value={reportUserID}
                onValueChange={setReportUserID}
                options={[
                  { value: '', label: 'All Members' },
                  ...members.map((member) => ({ value: member.id, label: member.name })),
                ]}
              />
            </label>
          )}
          <label>
            Project
            <SingleSelect
              label="Project"
              value={reportProjectID}
              onValueChange={setReportProjectID}
              options={[
                { value: '', label: 'All projects' },
                ...projects.map((project) => ({ value: project.id, label: project.name })),
              ]}
            />
          </label>
          <label>
            Tickets <small>match any selected ticket</small>
            <TicketPicker
              organizationID={organizationID}
              value={reportTicketIDs}
              onChange={setReportTicketIDs}
            />
          </label>
        </div>
        <fieldset>
          <legend>
            Tags <small>match any selected tag</small>
          </legend>
          <TagMultiSelect
            tags={tags}
            selectedIds={reportTagIDs}
            onToggle={toggleReportTag}
            showRecentByDefault
          />
        </fieldset>
        {reportView === 'summary' && (
          <fieldset>
            <legend>
              Group by <small>selected order defines the hierarchy</small>
            </legend>
            <div className="group-picker">
              {groupOptions.map((dimension) => {
                const index = reportGroupBy.indexOf(dimension)
                return (
                  <div className={`group-choice ${index >= 0 ? 'selected' : ''}`} key={dimension}>
                    <button
                      type="button"
                      className="group-toggle"
                      onClick={() => toggleReportGroup(dimension)}
                    >
                      {index >= 0 ? `${index + 1}. ` : ''}
                      {dimension}
                    </button>
                    {index >= 0 && (
                      <span>
                        <button
                          type="button"
                          aria-label={`Move ${dimension} earlier`}
                          disabled={index === 0}
                          onClick={() => moveReportGroup(index, -1)}
                        >
                          ↑
                        </button>
                        <button
                          type="button"
                          aria-label={`Move ${dimension} later`}
                          disabled={index === reportGroupBy.length - 1}
                          onClick={() => moveReportGroup(index, 1)}
                        >
                          ↓
                        </button>
                      </span>
                    )}
                  </div>
                )
              })}
            </div>
            {reportGroupBy.includes('date') && (
              <label className="date-grouping">
                Date grouping
                <SingleSelect
                  label="Date grouping"
                  value={reportDateGrouping}
                  onValueChange={(value) =>
                    setReportDateGrouping(value as 'day' | 'week' | 'month')
                  }
                  options={[
                    { value: 'day', label: 'Day' },
                    { value: 'week', label: 'Week (Mon–Sun)' },
                    { value: 'month', label: 'Month' },
                  ]}
                />
              </label>
            )}
          </fieldset>
        )}
        <div className="report-actions">
          <button
            disabled={
              reportLoading ||
              !reportStartDate ||
              !reportEndDate ||
              (reportView === 'summary' && reportGroupBy.length === 0)
            }
            onClick={() => void loadReport(0)}
          >
            {reportLoading ? 'Loading…' : 'Run report'}
          </button>
          <button
            className="secondary"
            disabled={reportLoading}
            onClick={() => void downloadReport('detailed')}
          >
            Download detailed CSV
          </button>
          <button
            className="secondary"
            disabled={reportLoading || reportGroupBy.length === 0}
            onClick={() => void downloadReport('summary')}
          >
            Download summary CSV
          </button>
        </div>
      </section>
      <section className="card report-results">
        <div className="section-heading">
          <div>
            <h2>{reportView === 'detailed' ? 'Detailed results' : 'Summary results'}</h2>
            <p>
              {activeReport
                ? `${activeReport.startDate} – ${activeReport.endDate} · ${activeReport.timezone}`
                : 'Choose filters and run the report.'}
            </p>
          </div>
          {activeReport && (
            <div className="report-total">
              <small>Total time</small>
              <strong>{hoursMinutes(activeReport.totalDurationSeconds)}</strong>
            </div>
          )}
        </div>
        {reportView === 'detailed' ? (
          report?.items.length ? (
            <div className="entry-list">
              {report.items.map((entry) => (
                <div className="entry-row completed-entry-row" key={entry.id}>
                  <div>
                    <strong>{entry.description}</strong>
                    <small>
                      {entry.projectName}
                      {entry.tickets.length
                        ? ` · ${entry.tickets.map((ticket) => ticket.reference).join(', ')}`
                        : ''}{' '}
                      · {report.scope === 'organization' && <>{entry.userName} · </>}
                      {reportDateTime(entry.startedAt, report.timezone)} –{' '}
                      {reportDateTime(entry.endedAt, report.timezone)}
                    </small>
                    {entry.tags.length > 0 && (
                      <span className="entry-tags">
                        {entry.tags.map((tag) => (
                          <em key={tag.id}>{tag.name}</em>
                        ))}
                      </span>
                    )}
                  </div>
                  <strong>{hoursMinutes(entry.reportDurationSeconds)}</strong>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState
              title={report ? 'No matching entries' : 'Report not run yet'}
              text={
                report
                  ? 'Adjust the filters to broaden the result.'
                  : 'Your filtered completed entries will appear here.'
              }
              compact
            />
          )
        ) : summary?.items.length ? (
          <div className="summary-table">
            {summary.items.map((row, index) => (
              <div
                className="summary-row"
                key={`${row.values.map((value) => value.key).join('-')}-${index}`}
              >
                <div>
                  {row.values.map((value) => (
                    <span key={`${value.dimension}-${value.key}`}>
                      <small>{value.dimension}</small>
                      <strong>{value.label}</strong>
                    </span>
                  ))}
                </div>
                <strong>{hoursMinutes(row.durationSeconds)}</strong>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState
            title={summary ? 'No matching groups' : 'Summary not run yet'}
            text={
              summary
                ? 'Adjust the filters or grouping.'
                : 'Select the grouping order and run the report.'
            }
            compact
          />
        )}
        {activeReport && activeReport.total > activeReport.limit && (
          <div className="pagination">
            <button
              className="secondary"
              disabled={reportLoading || activeReport.offset === 0}
              onClick={() => void loadReport(Math.max(0, activeReport.offset - activeReport.limit))}
            >
              Previous
            </button>
            <span>
              {activeReport.offset + 1}–
              {Math.min(activeReport.offset + activeReport.limit, activeReport.total)} of{' '}
              {activeReport.total}
            </span>
            <button
              className="secondary"
              disabled={
                reportLoading || activeReport.offset + activeReport.limit >= activeReport.total
              }
              onClick={() => void loadReport(activeReport.offset + activeReport.limit)}
            >
              Next
            </button>
          </div>
        )}
      </section>
    </div>
  )
}
