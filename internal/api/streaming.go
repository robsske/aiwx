package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rverhe/aiwx/internal/repository"
)

// SSE event types for real-time streaming.
const (
	SSEEventJobStatus  = "job.status"
	SSEEventJobLog     = "job.log"
	SSEEventJobEvent   = "job.event"
	SSEEventHeartbeat  = "heartbeat"
)

// SSEMessage represents a server-sent event.
type SSEMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
	ID    string      `json:"id,omitempty"`
}

// handleJobStream streams job events via Server-Sent Events.
func (d *Deps) handleJobStream(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job ID")
		return
	}

	// Verify job exists
	job, err := d.Repos.Jobs.GetByID(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
		return
	}

	// Set SSE headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial job status
	sendSSE(w, flusher, SSEMessage{
		Event: SSEEventJobStatus,
		Data: map[string]any{
			"job_id": job.ID,
			"status": job.Status,
		},
	})

	// Stream events
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastEventID := int64(0)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			events, err := d.Repos.JobEvents.ListByJobSince(r.Context(), jobID, lastEventID, 50)
			if err != nil {
				continue
			}

			for _, event := range events {
				sendSSE(w, flusher, SSEMessage{
					Event: SSEEventJobEvent,
					Data:  event,
					ID:    fmt.Sprintf("%d", event.ID),
				})
				lastEventID = event.ID
			}

			sendSSE(w, flusher, SSEMessage{
				Event: SSEEventHeartbeat,
				Data:  map[string]string{"timestamp": time.Now().UTC().Format(time.RFC3339)},
			})

			if job.Status.Terminal() {
				sendSSE(w, flusher, SSEMessage{
					Event: SSEEventJobStatus,
					Data: map[string]any{
						"job_id": job.ID,
						"status": job.Status,
						"final":  true,
					},
				})
				return
			}

			job, err = d.Repos.Jobs.GetByID(r.Context(), jobID)
			if err != nil {
				return
			}
		}
	}
}

// handleJobLogStream streams job log output via SSE.
func (d *Deps) handleJobLogStream(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job ID")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastSequence := int64(0)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			chunks, err := d.Repos.JobLogs.ListSince(r.Context(), jobID, lastSequence, 100)
			if err != nil {
				continue
			}

			for _, chunk := range chunks {
				sendSSE(w, flusher, SSEMessage{
					Event: SSEEventJobLog,
					Data:  chunk,
					ID:    fmt.Sprintf("%d", chunk.Sequence),
				})
				lastSequence = chunk.Sequence
			}

			job, err := d.Repos.Jobs.GetByID(r.Context(), jobID)
			if err != nil {
				return
			}
			if job.Status.Terminal() && len(chunks) == 0 {
				return
			}
		}
	}
}

// sendSSE sends a server-sent event.
func sendSSE(w http.ResponseWriter, flusher http.Flusher, msg SSEMessage) {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return
	}

	if msg.Event != "" {
		fmt.Fprintf(w, "event: %s\n", msg.Event)
	}
	if msg.ID != "" {
		fmt.Fprintf(w, "id: %s\n", msg.ID)
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// Ensure repository types are referenced
var _ = repository.JobLogChunk{}
