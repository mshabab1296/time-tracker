package httpserver

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}
func (writer *statusWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}
func (writer *statusWriter) code() int {
	if writer.status == 0 {
		return http.StatusOK
	}
	return writer.status
}
func (writer *statusWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

var latencyBuckets = [...]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.3, 0.5, 1, 2, 5}

type requestMetrics struct {
	mutex   sync.Mutex
	count   [6]uint64
	buckets [10]uint64
	total   uint64
	seconds float64
}

func newRequestMetrics() *requestMetrics { return &requestMetrics{} }
func (metrics *requestMetrics) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/metrics" {
			next.ServeHTTP(writer, request)
			return
		}
		started := time.Now()
		response := &statusWriter{ResponseWriter: writer}
		defer func() {
			elapsed := time.Since(started).Seconds()
			status := response.code()
			metrics.mutex.Lock()
			defer metrics.mutex.Unlock()
			metrics.total++
			metrics.seconds += elapsed
			if status/100 >= 1 && status/100 <= 5 {
				metrics.count[status/100]++
			}
			for index, bound := range latencyBuckets {
				if elapsed <= bound {
					metrics.buckets[index]++
				}
			}
		}()
		next.ServeHTTP(response, request)
	})
}
func (metrics *requestMetrics) serveHTTP(writer http.ResponseWriter, _ *http.Request) {
	metrics.mutex.Lock()
	counts, buckets, total, seconds := metrics.count, metrics.buckets, metrics.total, metrics.seconds
	metrics.mutex.Unlock()
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = fmt.Fprintln(writer, "# HELP timetracker_http_requests_total Completed requests by status class.\n# TYPE timetracker_http_requests_total counter")
	for status := 1; status <= 5; status++ {
		_, _ = fmt.Fprintf(writer, "timetracker_http_requests_total{status_class=\"%dxx\"} %d\n", status, counts[status])
	}
	_, _ = fmt.Fprintln(writer, "# HELP timetracker_http_request_duration_seconds Request duration excluding the metrics endpoint.\n# TYPE timetracker_http_request_duration_seconds histogram")
	for index, bound := range latencyBuckets {
		_, _ = fmt.Fprintf(writer, "timetracker_http_request_duration_seconds_bucket{le=\"%g\"} %d\n", bound, buckets[index])
	}
	_, _ = fmt.Fprintf(writer, "timetracker_http_request_duration_seconds_bucket{le=\"+Inf\"} %d\ntimetracker_http_request_duration_seconds_sum %g\ntimetracker_http_request_duration_seconds_count %d\n", total, seconds, total)
}
