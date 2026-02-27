package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"github.com/fhsinchy/bolt/internal/event"
)

const wsWriteTimeout = 5 * time.Second

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("websocket accept", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// We don't read from the client; close the read side.
	conn.CloseRead(r.Context())

	ch, subID := s.bus.Subscribe()
	defer s.bus.Unsubscribe(subID)

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}

			msg := eventToWSMessage(evt)
			if msg == nil {
				continue
			}

			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			writeCtx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
			err = conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// eventToWSMessage converts an event bus event to a JSON-serializable map
// suitable for WebSocket transmission. Returns nil for unrecognized events.
func eventToWSMessage(evt event.Event) map[string]any {
	switch e := evt.(type) {
	case event.Progress:
		return map[string]any{
			"type":        "progress",
			"download_id": e.DownloadID,
			"downloaded":  e.Downloaded,
			"total_size":  e.TotalSize,
			"speed":       e.Speed,
			"eta":         e.ETA,
			"status":      e.Status,
		}
	case event.DownloadAdded:
		return map[string]any{
			"type":        "download_added",
			"download_id": e.DownloadID,
			"filename":    e.Filename,
			"total_size":  e.TotalSize,
		}
	case event.DownloadCompleted:
		return map[string]any{
			"type":        "download_completed",
			"download_id": e.DownloadID,
			"filename":    e.Filename,
		}
	case event.DownloadFailed:
		return map[string]any{
			"type":        "download_failed",
			"download_id": e.DownloadID,
			"error":       e.Error,
		}
	case event.DownloadRemoved:
		return map[string]any{
			"type":        "download_removed",
			"download_id": e.DownloadID,
		}
	case event.DownloadPaused:
		return map[string]any{
			"type":        "download_paused",
			"download_id": e.DownloadID,
		}
	case event.DownloadResumed:
		return map[string]any{
			"type":        "download_resumed",
			"download_id": e.DownloadID,
		}
	case event.RefreshNeeded:
		return map[string]any{
			"type":        "refresh_needed",
			"download_id": e.DownloadID,
		}
	default:
		return nil
	}
}
