package reporting

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fortune-tech/time-tracker/apps/api/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maximumReportEntries = 10000

var validGroupDimensions = map[string]bool{"member": true, "project": true, "ticket": true, "tag": true, "date": true}

type Handler struct{ Database *pgxpool.Pool }

type reportEntry struct {
	ID, OrganizationID, UserID, ProjectID          uuid.UUID
	UserName, ProjectName, Description, SourceType string
	StartedAt, EndedAt                             time.Time
	DurationSeconds                                int64
	TagIDs                                         []uuid.UUID
	TagNames                                       []string
	TicketIDs                                      []uuid.UUID
	TicketReferences, TicketTitles                 []string
}

type timerEvent struct {
	Type string
	At   time.Time
}

type interval struct{ Start, End time.Time }

type reportRequest struct {
	OrganizationID          uuid.UUID
	Scope, Timezone         string
	Location                *time.Location
	Start, End, RangeEnd    time.Time
	TargetUserID, ProjectID *uuid.UUID
	TicketIDs               []uuid.UUID
	TagIDs                  []uuid.UUID
	Limit, Offset           int
	GroupBy                 []string
	DateGrouping            string
}

type reportData struct {
	Request  reportRequest
	Entries  []reportEntry
	Events   map[uuid.UUID][]timerEvent
	Duration map[uuid.UUID]int64
	Total    int64
}

type groupValue struct {
	Dimension string `json:"dimension"`
	Key       string `json:"key"`
	Label     string `json:"label"`
}

type summaryRow struct {
	Values          []groupValue `json:"values"`
	DurationSeconds int64        `json:"durationSeconds"`
}

func (handler Handler) Entries(writer http.ResponseWriter, request *http.Request) {
	data, ok := handler.buildReport(writer, request, false)
	if !ok {
		return
	}
	items := make([]map[string]any, 0, min(data.Request.Limit, len(data.Entries)))
	for index, entry := range data.Entries {
		if index >= data.Request.Offset && index < data.Request.Offset+data.Request.Limit {
			items = append(items, entryResponse(entry, data.Duration[entry.ID]))
		}
	}
	respond(writer, http.StatusOK, map[string]any{
		"items": items, "total": len(data.Entries), "totalDurationSeconds": data.Total,
		"limit": data.Request.Limit, "offset": data.Request.Offset, "timezone": data.Request.Timezone,
		"scope": data.Request.Scope, "startDate": data.Request.Start.Format("2006-01-02"),
		"endDate": data.Request.End.Format("2006-01-02"),
	})
}

func (handler Handler) Summary(writer http.ResponseWriter, request *http.Request) {
	data, ok := handler.buildReport(writer, request, true)
	if !ok {
		return
	}
	rows := summarize(data)
	start := min(data.Request.Offset, len(rows))
	end := min(start+data.Request.Limit, len(rows))
	respond(writer, http.StatusOK, map[string]any{
		"items": rows[start:end], "total": len(rows), "totalDurationSeconds": data.Total,
		"limit": data.Request.Limit, "offset": data.Request.Offset, "timezone": data.Request.Timezone,
		"scope": data.Request.Scope, "startDate": data.Request.Start.Format("2006-01-02"),
		"endDate": data.Request.End.Format("2006-01-02"), "groupBy": data.Request.GroupBy,
		"dateGrouping": data.Request.DateGrouping,
	})
}

func (handler Handler) Export(writer http.ResponseWriter, request *http.Request) {
	format := request.URL.Query().Get("format")
	if format != "detailed" && format != "summary" {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Export format must be detailed or summary.")
		return
	}
	data, ok := handler.buildReport(writer, request, format == "summary")
	if !ok {
		return
	}
	filename := fmt.Sprintf("time-report-%s-%s-to-%s.csv", format, data.Request.Start.Format("2006-01-02"), data.Request.End.Format("2006-01-02"))
	writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	writer.WriteHeader(http.StatusOK)
	csvWriter := csv.NewWriter(writer)
	if format == "detailed" {
		writeDetailedCSV(csvWriter, data)
	} else {
		writeSummaryCSV(csvWriter, data, summarize(data))
	}
	csvWriter.Flush()
}

func (handler Handler) buildReport(writer http.ResponseWriter, request *http.Request, requireGrouping bool) (reportData, bool) {
	actorID, ok := auth.AuthenticatedUserIDForRead(writer, request, handler.Database)
	if !ok {
		return reportData{}, false
	}
	input, ok := handler.parseRequest(writer, request, actorID, requireGrouping)
	if !ok {
		return reportData{}, false
	}
	entries, err := handler.loadEntries(request, input)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the report.")
		return reportData{}, false
	}
	if len(entries) > maximumReportEntries {
		respondError(writer, http.StatusUnprocessableEntity, "REPORT_LIMIT_EXCEEDED", "Narrow the filters to 10,000 completed entries or fewer.")
		return reportData{}, false
	}
	events, err := handler.loadTimerEvents(request, entries)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to calculate the report.")
		return reportData{}, false
	}
	durations := make(map[uuid.UUID]int64, len(entries))
	var total int64
	for _, entry := range entries {
		for _, active := range entryIntervals(entry, events[entry.ID], input.Start.UTC(), input.RangeEnd.UTC()) {
			durations[entry.ID] += int64(active.End.Sub(active.Start).Seconds())
		}
		total += durations[entry.ID]
	}
	return reportData{Request: input, Entries: entries, Events: events, Duration: durations, Total: total}, true
}

func (handler Handler) parseRequest(writer http.ResponseWriter, request *http.Request, actorID uuid.UUID, requireGrouping bool) (reportRequest, bool) {
	query := request.URL.Query()
	organizationID, err := uuid.Parse(query.Get("organizationId"))
	if err != nil || organizationID == uuid.Nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "A valid organization identifier is required.")
		return reportRequest{}, false
	}
	var role, organizationTimezone, userTimezone string
	err = handler.Database.QueryRow(request.Context(), `
		SELECT m.role, o.timezone, u.timezone FROM memberships m
		JOIN organizations o ON o.id = m.organization_id JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1 AND m.user_id = $2`, organizationID, actorID).Scan(&role, &organizationTimezone, &userTimezone)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "You do not belong to this organization.")
		return reportRequest{}, false
	}
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to load the report.")
		return reportRequest{}, false
	}
	scope := query.Get("scope")
	if scope == "" {
		scope = "personal"
	}
	if scope != "personal" && scope != "organization" {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Report scope must be personal or organization.")
		return reportRequest{}, false
	}
	if scope == "organization" && role != "ADMIN" {
		respondError(writer, http.StatusForbidden, "FORBIDDEN", "Only an Admin can view an organization report.")
		return reportRequest{}, false
	}
	timezone := userTimezone
	var targetUserID *uuid.UUID
	if scope == "personal" {
		targetUserID = &actorID
	} else {
		timezone = organizationTimezone
		if rawUserID := query.Get("userId"); rawUserID != "" {
			parsed, parseErr := uuid.Parse(rawUserID)
			if parseErr != nil || parsed == uuid.Nil {
				respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "A valid member identifier is required.")
				return reportRequest{}, false
			}
			targetUserID = &parsed
		}
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "INTERNAL_ERROR", "The report timezone is invalid.")
		return reportRequest{}, false
	}
	now := time.Now().In(location)
	start, err := parseDate(query.Get("startDate"), time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location), location)
	if err != nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Start date must use YYYY-MM-DD.")
		return reportRequest{}, false
	}
	end, err := parseDate(query.Get("endDate"), now, location)
	if err != nil || end.Before(start) {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "End date must use YYYY-MM-DD and not precede the start date.")
		return reportRequest{}, false
	}
	projectID, ok := optionalUUID(writer, query.Get("projectId"), "project")
	if !ok {
		return reportRequest{}, false
	}
	ticketIDs, ok := parseFilterIDs(writer, query["ticketId"], "Ticket")
	if !ok {
		return reportRequest{}, false
	}
	tagIDs, ok := parseTagIDs(writer, query["tagId"])
	if !ok {
		return reportRequest{}, false
	}
	limit, offset, ok := pagination(writer, query.Get("limit"), query.Get("offset"))
	if !ok {
		return reportRequest{}, false
	}
	groupBy, ok := parseGroupBy(writer, query["groupBy"], scope, requireGrouping)
	if !ok {
		return reportRequest{}, false
	}
	dateGrouping := query.Get("dateGrouping")
	if dateGrouping == "" {
		dateGrouping = "day"
	}
	if dateGrouping != "day" && dateGrouping != "week" && dateGrouping != "month" {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Date grouping must be day, week, or month.")
		return reportRequest{}, false
	}
	return reportRequest{OrganizationID: organizationID, Scope: scope, Timezone: timezone, Location: location, Start: start, End: end, RangeEnd: end.AddDate(0, 0, 1), TargetUserID: targetUserID, ProjectID: projectID, TicketIDs: ticketIDs, TagIDs: tagIDs, Limit: limit, Offset: offset, GroupBy: groupBy, DateGrouping: dateGrouping}, true
}

func parseGroupBy(writer http.ResponseWriter, raw []string, scope string, required bool) ([]string, bool) {
	result := make([]string, 0)
	seen := make(map[string]bool)
	for _, item := range raw {
		for _, dimension := range strings.Split(item, ",") {
			dimension = strings.ToLower(strings.TrimSpace(dimension))
			if !validGroupDimensions[dimension] || seen[dimension] || scope == "personal" && dimension == "member" {
				respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Group dimensions must be unique supported values for this report scope.")
				return nil, false
			}
			seen[dimension] = true
			result = append(result, dimension)
		}
	}
	if required && len(result) == 0 {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Select at least one summary grouping.")
		return nil, false
	}
	return result, true
}

func (handler Handler) loadEntries(request *http.Request, input reportRequest) ([]reportEntry, error) {
	arguments := []any{input.OrganizationID, input.Start.UTC(), input.RangeEnd.UTC()}
	filters := []string{"te.organization_id = $1", "te.status = 'STOPPED'", "te.started_at < $3", "te.ended_at > $2"}
	if input.TargetUserID != nil {
		arguments = append(arguments, *input.TargetUserID)
		filters = append(filters, fmt.Sprintf("te.user_id = $%d", len(arguments)))
	}
	if input.ProjectID != nil {
		arguments = append(arguments, *input.ProjectID)
		filters = append(filters, fmt.Sprintf("te.project_id = $%d", len(arguments)))
	}
	if len(input.TicketIDs) > 0 {
		arguments = append(arguments, input.TicketIDs)
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM time_entry_tickets selected_tickets WHERE selected_tickets.time_entry_id = te.id AND selected_tickets.ticket_id = ANY($%d))", len(arguments)))
	}
	if len(input.TagIDs) > 0 {
		arguments = append(arguments, input.TagIDs)
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM time_entry_tags selected_tags WHERE selected_tags.time_entry_id = te.id AND selected_tags.tag_id = ANY($%d))", len(arguments)))
	}
	statement := `SELECT te.id, te.organization_id, te.user_id, u.name, te.project_id, p.name, te.description, te.source_type,
		te.started_at, te.ended_at, te.duration_seconds,
		ARRAY(SELECT ticket.id FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
		ARRAY(SELECT ticket.reference FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
		ARRAY(SELECT ticket.title FROM time_entry_tickets teticket JOIN tickets ticket ON ticket.id = teticket.ticket_id WHERE teticket.time_entry_id = te.id ORDER BY ticket.reference),
		COALESCE(array_agg(t.id) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::uuid[]),
		COALESCE(array_agg(t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
		FROM time_entries te JOIN users u ON u.id = te.user_id JOIN projects p ON p.id = te.project_id
		LEFT JOIN time_entry_tags tet ON tet.time_entry_id = te.id LEFT JOIN tags t ON t.id = tet.tag_id
		WHERE ` + strings.Join(filters, " AND ") + ` GROUP BY te.id, u.name, p.name ORDER BY te.started_at DESC LIMIT 10001`
	rows, err := handler.Database.Query(request.Context(), statement, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]reportEntry, 0)
	for rows.Next() {
		entry := reportEntry{TagIDs: make([]uuid.UUID, 0), TagNames: make([]string, 0)}
		if err := rows.Scan(&entry.ID, &entry.OrganizationID, &entry.UserID, &entry.UserName, &entry.ProjectID, &entry.ProjectName, &entry.Description, &entry.SourceType, &entry.StartedAt, &entry.EndedAt, &entry.DurationSeconds, &entry.TicketIDs, &entry.TicketReferences, &entry.TicketTitles, &entry.TagIDs, &entry.TagNames); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (handler Handler) loadTimerEvents(request *http.Request, entries []reportEntry) (map[uuid.UUID][]timerEvent, error) {
	ids := make([]uuid.UUID, 0)
	for _, entry := range entries {
		if entry.SourceType == "TIMER" {
			ids = append(ids, entry.ID)
		}
	}
	result := make(map[uuid.UUID][]timerEvent)
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := handler.Database.Query(request.Context(), `SELECT time_entry_id, event_type, occurred_at FROM timer_events WHERE time_entry_id = ANY($1) ORDER BY time_entry_id, sequence_number`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var entryID uuid.UUID
		var event timerEvent
		if err := rows.Scan(&entryID, &event.Type, &event.At); err != nil {
			return nil, err
		}
		result[entryID] = append(result[entryID], event)
	}
	return result, rows.Err()
}

func entryIntervals(entry reportEntry, events []timerEvent, rangeStart, rangeEnd time.Time) []interval {
	if entry.SourceType != "TIMER" {
		return clippedIntervals([]interval{{Start: entry.StartedAt, End: entry.EndedAt}}, rangeStart, rangeEnd)
	}
	result := make([]interval, 0)
	var runningFrom *time.Time
	for _, event := range events {
		switch event.Type {
		case "START", "RESUME":
			at := event.At
			runningFrom = &at
		case "PAUSE", "STOP":
			if runningFrom != nil {
				result = append(result, interval{Start: *runningFrom, End: event.At})
				runningFrom = nil
			}
		}
	}
	return clippedIntervals(result, rangeStart, rangeEnd)
}

func clippedIntervals(values []interval, rangeStart, rangeEnd time.Time) []interval {
	result := make([]interval, 0, len(values))
	for _, value := range values {
		if value.Start.Before(rangeStart) {
			value.Start = rangeStart
		}
		if value.End.After(rangeEnd) {
			value.End = rangeEnd
		}
		if value.End.After(value.Start) {
			result = append(result, value)
		}
	}
	return result
}

func splitAtMidnight(value interval, location *time.Location) []interval {
	result := make([]interval, 0, 2)
	for cursor := value.Start; cursor.Before(value.End); {
		local := cursor.In(location)
		nextMidnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location)
		segmentEnd := value.End
		if nextMidnight.Before(segmentEnd) {
			segmentEnd = nextMidnight
		}
		result = append(result, interval{Start: cursor, End: segmentEnd})
		cursor = segmentEnd
	}
	return result
}

func summarize(data reportData) []summaryRow {
	totals := make(map[string]*summaryRow)
	for _, entry := range data.Entries {
		for _, active := range entryIntervals(entry, data.Events[entry.ID], data.Request.Start.UTC(), data.Request.RangeEnd.UTC()) {
			for _, segment := range splitAtMidnight(active, data.Request.Location) {
				combinations := groupCombinations(entry, segment.Start.In(data.Request.Location), data.Request.GroupBy, data.Request.DateGrouping)
				seconds := int64(segment.End.Sub(segment.Start).Seconds())
				for _, values := range combinations {
					parts := make([]string, len(values))
					for index, value := range values {
						parts[index] = value.Dimension + "=" + value.Key
					}
					key := strings.Join(parts, "\x1f")
					if totals[key] == nil {
						copyValues := append([]groupValue(nil), values...)
						totals[key] = &summaryRow{Values: copyValues}
					}
					totals[key].DurationSeconds += seconds
				}
			}
		}
	}
	rows := make([]summaryRow, 0, len(totals))
	for _, row := range totals {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool { return summarySortKey(rows[i]) < summarySortKey(rows[j]) })
	return rows
}

func groupCombinations(entry reportEntry, localTime time.Time, dimensions []string, dateGrouping string) [][]groupValue {
	combinations := [][]groupValue{{}}
	for _, dimension := range dimensions {
		values := valuesForDimension(entry, localTime, dimension, dateGrouping)
		next := make([][]groupValue, 0, len(combinations)*len(values))
		for _, combination := range combinations {
			for _, value := range values {
				next = append(next, append(append([]groupValue(nil), combination...), value))
			}
		}
		combinations = next
	}
	return combinations
}

func valuesForDimension(entry reportEntry, localTime time.Time, dimension, dateGrouping string) []groupValue {
	switch dimension {
	case "member":
		return []groupValue{{Dimension: dimension, Key: entry.UserID.String(), Label: entry.UserName}}
	case "project":
		return []groupValue{{Dimension: dimension, Key: entry.ProjectID.String(), Label: entry.ProjectName}}
	case "ticket":
		if len(entry.TicketIDs) == 0 {
			return []groupValue{{Dimension: dimension, Key: "no-ticket", Label: "No ticket"}}
		}
		values := make([]groupValue, len(entry.TicketIDs))
		for index, id := range entry.TicketIDs {
			values[index] = groupValue{Dimension: dimension, Key: id.String(), Label: entry.TicketReferences[index]}
		}
		return values
	case "tag":
		if len(entry.TagIDs) == 0 {
			return []groupValue{{Dimension: dimension, Key: "untagged", Label: "Untagged"}}
		}
		values := make([]groupValue, len(entry.TagIDs))
		for index, id := range entry.TagIDs {
			values[index] = groupValue{Dimension: dimension, Key: id.String(), Label: entry.TagNames[index]}
		}
		return values
	default:
		key, label := dateBucket(localTime, dateGrouping)
		return []groupValue{{Dimension: "date", Key: key, Label: label}}
	}
}

func dateBucket(value time.Time, grouping string) (string, string) {
	switch grouping {
	case "week":
		daysFromMonday := (int(value.Weekday()) + 6) % 7
		start := time.Date(value.Year(), value.Month(), value.Day()-daysFromMonday, 0, 0, 0, 0, value.Location())
		end := start.AddDate(0, 0, 6)
		return start.Format("2006-01-02"), start.Format("02 Jan 2006") + " – " + end.Format("02 Jan 2006")
	case "month":
		start := time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
		return start.Format("2006-01"), start.Format("January 2006")
	default:
		return value.Format("2006-01-02"), value.Format("02 Jan 2006")
	}
}

func summarySortKey(row summaryRow) string {
	parts := make([]string, len(row.Values))
	for index, value := range row.Values {
		parts[index] = value.Key
	}
	return strings.Join(parts, "\x1f")
}

func writeDetailedCSV(writer *csv.Writer, data reportData) {
	_ = writer.Write([]string{"Member", "Project", "Ticket", "Ticket title", "Description", "Tags", "Source", "Start", "End", "Duration seconds", "Duration"})
	for _, entry := range data.Entries {
		seconds := data.Duration[entry.ID]
		_ = writer.Write([]string{entry.UserName, entry.ProjectName, strings.Join(entry.TicketReferences, ", "), strings.Join(entry.TicketTitles, ", "), entry.Description, strings.Join(entry.TagNames, ", "), entry.SourceType, entry.StartedAt.In(data.Request.Location).Format("2006-01-02 15:04:05 MST"), entry.EndedAt.In(data.Request.Location).Format("2006-01-02 15:04:05 MST"), strconv.FormatInt(seconds, 10), formatDuration(seconds)})
	}
}

func writeSummaryCSV(writer *csv.Writer, data reportData, rows []summaryRow) {
	header := make([]string, len(data.Request.GroupBy))
	for index, dimension := range data.Request.GroupBy {
		header[index] = strings.ToUpper(dimension[:1]) + dimension[1:]
		if dimension == "date" {
			header[index] += " (" + data.Request.DateGrouping + ")"
		}
	}
	header = append(header, "Duration seconds", "Duration")
	_ = writer.Write(header)
	for _, row := range rows {
		record := make([]string, 0, len(row.Values)+2)
		for _, value := range row.Values {
			record = append(record, value.Label)
		}
		record = append(record, strconv.FormatInt(row.DurationSeconds, 10), formatDuration(row.DurationSeconds))
		_ = writer.Write(record)
	}
}

func parseDate(value string, fallback time.Time, location *time.Location) (time.Time, error) {
	if value == "" {
		return time.Date(fallback.Year(), fallback.Month(), fallback.Day(), 0, 0, 0, 0, location), nil
	}
	return time.ParseInLocation("2006-01-02", value, location)
}

func parseTagIDs(writer http.ResponseWriter, raw []string) ([]uuid.UUID, bool) {
	return parseFilterIDs(writer, raw, "Tag")
}

func parseFilterIDs(writer http.ResponseWriter, raw []string, name string) ([]uuid.UUID, bool) {
	result := make([]uuid.UUID, 0, len(raw))
	seen := make(map[uuid.UUID]bool)
	for _, item := range raw {
		for _, value := range strings.Split(item, ",") {
			parsed, err := uuid.Parse(strings.TrimSpace(value))
			if err != nil || parsed == uuid.Nil {
				respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", name+" filters must contain valid identifiers.")
				return nil, false
			}
			if !seen[parsed] {
				seen[parsed] = true
				result = append(result, parsed)
			}
		}
	}
	return result, true
}

func optionalUUID(writer http.ResponseWriter, value, name string) (*uuid.UUID, bool) {
	if value == "" {
		return nil, true
	}
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("A valid %s identifier is required.", name))
		return nil, false
	}
	return &parsed, true
}

func pagination(writer http.ResponseWriter, rawLimit, rawOffset string) (int, int, bool) {
	limit, offset := 25, 0
	if rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Limit must be between 1 and 100.")
			return 0, 0, false
		}
		limit = parsed
	}
	if rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			respondError(writer, http.StatusBadRequest, "VALIDATION_ERROR", "Offset must be zero or greater.")
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
}

func intervalDuration(start, end, rangeStart, rangeEnd time.Time) int64 {
	values := clippedIntervals([]interval{{Start: start, End: end}}, rangeStart, rangeEnd)
	if len(values) == 0 {
		return 0
	}
	return int64(values[0].End.Sub(values[0].Start).Seconds())
}

func activeDuration(events []timerEvent, rangeStart, rangeEnd time.Time) int64 {
	entry := reportEntry{SourceType: "TIMER"}
	var total int64
	for _, value := range entryIntervals(entry, events, rangeStart, rangeEnd) {
		total += int64(value.End.Sub(value.Start).Seconds())
	}
	return total
}

func formatDuration(seconds int64) string {
	return fmt.Sprintf("%dh %dm", seconds/3600, seconds%3600/60)
}

func entryResponse(entry reportEntry, reportDuration int64) map[string]any {
	tags := make([]map[string]any, 0, len(entry.TagIDs))
	for index, id := range entry.TagIDs {
		tags = append(tags, map[string]any{"id": id, "name": entry.TagNames[index]})
	}
	tickets := make([]map[string]any, 0, len(entry.TicketIDs))
	for index, id := range entry.TicketIDs {
		tickets = append(tickets, map[string]any{"id": id, "reference": entry.TicketReferences[index], "title": entry.TicketTitles[index]})
	}
	return map[string]any{"id": entry.ID, "organizationId": entry.OrganizationID, "userId": entry.UserID, "userName": entry.UserName, "projectId": entry.ProjectID, "projectName": entry.ProjectName, "description": entry.Description, "sourceType": entry.SourceType, "startedAt": entry.StartedAt, "endedAt": entry.EndedAt, "durationSeconds": entry.DurationSeconds, "reportDurationSeconds": reportDuration, "tickets": tickets, "tags": tags}
}

func respond(writer http.ResponseWriter, status int, data any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"data": data, "error": nil, "meta": map[string]any{}})
}

func respondError(writer http.ResponseWriter, status int, code, message string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"data": nil, "error": map[string]string{"code": code, "message": message}, "meta": map[string]any{}})
}
