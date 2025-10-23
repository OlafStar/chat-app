package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"chat-app-backend/utils"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

type LogEntry struct {
	Time      string `json:"time"`
	Method    string `json:"method"`
	URI       string `json:"uri"`
	Status    int    `json:"status"`
	Size      int    `json:"size"`
	Duration  string `json:"duration"`
	ClientIP  string `json:"client_ip"`
	UserAgent string `json:"user_agent"`
	Referer   string `json:"referer"`
	RequestID string `json:"request_id"`
}

func Logging() Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}

			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = generateRequestID()
			}
			w.Header().Set("X-Request-ID", reqID)

			next(rec, r)
			duration := time.Since(start)

			entry := LogEntry{
				Time:      start.Format(time.RFC3339),
				Method:    r.Method,
				URI:       r.URL.RequestURI(),
				Status:    rec.status,
				Size:      rec.size,
				Duration:  duration.String(),
				ClientIP:  utils.RealClientIP(r),
				UserAgent: r.UserAgent(),
				Referer:   r.Referer(),
				RequestID: reqID,
			}

			data, err := json.Marshal(entry)
			if err != nil {
				log.Printf("error marshaling log entry: %v", err)
				return
			}
			log.Println(string(data))
		}
	}
}

func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
}
