// Package middleware provides HTTP middlewares.
package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple in-memory rate limiter with cleanup.
type RateLimiter struct {
	requests       map[string][]time.Time
	mu             sync.RWMutex
	requestsPerMin int
	stopCleanup    chan struct{}
}

// NewRateLimiter creates a new rate limiter with periodic cleanup.
func NewRateLimiter(requestsPerMin int) *RateLimiter {
	rl := &RateLimiter{
		requests:       make(map[string][]time.Time),
		requestsPerMin: requestsPerMin,
		stopCleanup:    make(chan struct{}),
	}
	// Start cleanup goroutine
	go rl.cleanupLoop()
	return rl
}

// cleanupLoop periodically removes stale entries to prevent memory leaks.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes IPs with no recent requests.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-time.Minute)
	for ip, times := range rl.requests {
		// Filter to only recent requests
		filtered := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = filtered
		}
	}
}

// Stop stops the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// Middleware returns an HTTP middleware that rate limits requests.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for health checks
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}

		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-time.Minute)

		// Clean old requests for this IP
		reqs := rl.requests[clientIP]
		filtered := reqs[:0]
		for _, t := range reqs {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		rl.requests[clientIP] = filtered

		// Check limit
		if len(filtered) >= rl.requestsPerMin {
			rl.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests"}`))
			return
		}

		// Add current request
		rl.requests[clientIP] = append(rl.requests[clientIP], now)
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// Logger returns an HTTP middleware that logs requests.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"duration", time.Since(start).String(),
				"ip", r.RemoteAddr,
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Recoverer returns an HTTP middleware that recovers from panics.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("panic recovered", "error", err, "path", r.URL.Path)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"internal_error","message":"An unexpected error occurred"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
