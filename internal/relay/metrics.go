package relay

import (
	"fmt"
	"net/http"
	"time"

	"github.com/whaeuser/drop/internal/transfer"
)

// handleMetrics serves Prometheus text format without a client dependency.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	fileCount, fileBytes := s.fileStore.Stats()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	gauge := func(name, help string, value any) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %v\n", name, help, name, name, value)
	}
	counter := func(name, help string, value int64) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
	}

	fmt.Fprintf(w, "# HELP drop_build_info Build metadata.\n# TYPE drop_build_info gauge\ndrop_build_info{version=%q,protocol=\"%d\"} 1\n",
		s.version, int(transfer.ProtocolVersion))
	gauge("drop_uptime_seconds", "Seconds since the relay started.", int(time.Since(s.startedAt).Seconds()))
	gauge("drop_active_files", "Mailbox blobs currently held.", fileCount)
	gauge("drop_active_file_bytes", "Total bytes of mailbox blobs currently held.", fileBytes)
	gauge("drop_active_ws_rooms", "Live WebSocket rooms.", s.roomManager.ActiveRooms())
	counter("drop_uploads_total", "Mailbox uploads accepted.", s.uploadsTotal.Load())
	counter("drop_upload_bytes_total", "Bytes accepted into the mailbox.", s.uploadBytesTotal.Load())
	counter("drop_downloads_total", "Mailbox downloads served.", s.downloadsTotal.Load())
	counter("drop_download_bytes_total", "Bytes served from the mailbox.", s.downloadBytesTotal.Load())
	counter("drop_ws_connections_total", "WebSocket clients that joined a room.", s.wsConnectionsTotal.Load())
	counter("drop_rate_limited_total", "Requests rejected by the per-IP rate limit.", s.rateLimitedTotal.Load())
	counter("drop_expired_files_total", "Mailbox blobs deleted by expiry.", s.fileStore.ExpiredTotal())
}
