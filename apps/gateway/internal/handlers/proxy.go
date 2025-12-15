package handlers

import (
	"net/http"
)

// ProxyWorker proxies requests to the Python worker service.
// It relies on the workerProxy initialized in New().
func (h *Handler) ProxyWorker(w http.ResponseWriter, r *http.Request) {
	if h.workerProxy == nil {
		h.writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Worker service not configured")
		return
	}

	// Logging could be enhanced here to track proxied requests
	h.log.Info("proxying request to worker",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	// Proxy the request
	h.workerProxy.ServeHTTP(w, r)
}
