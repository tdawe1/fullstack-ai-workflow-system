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
			_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests"}`))
			return
		}

		// Add current request
		rl.requests[clientIP] = append(rl.requests[clientIP], now)
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// MFALimiter implements aggressive rate limiting for MFA verification endpoints.
// Limits to 5 attempts per 5 minutes per IP to prevent brute-force attacks on TOTP.
type MFALimiter struct {
	attempts       map[string][]time.Time
	mu             sync.RWMutex
	maxAttempts    int           // Max attempts in window
	windowDuration time.Duration // Time window
	stopCleanup    chan struct{}
}

// NewMFALimiter creates a new MFA-specific rate limiter.
// Default: 5 attempts per 5 minutes.
func NewMFALimiter() *MFALimiter {
	ml := &MFALimiter{
		attempts:       make(map[string][]time.Time),
		maxAttempts:    5,
		windowDuration: 5 * time.Minute,
		stopCleanup:    make(chan struct{}),
	}
	go ml.cleanupLoop()
	return ml
}

func (ml *MFALimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ml.cleanup()
		case <-ml.stopCleanup:
			return
		}
	}
}

func (ml *MFALimiter) cleanup() {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	cutoff := time.Now().Add(-ml.windowDuration)
	for ip, times := range ml.attempts {
		filtered := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			delete(ml.attempts, ip)
		} else {
			ml.attempts[ip] = filtered
		}
	}
}

// Stop stops the cleanup goroutine.
func (ml *MFALimiter) Stop() {
	close(ml.stopCleanup)
}

// Middleware returns an HTTP middleware that applies MFA-specific rate limiting.
func (ml *MFALimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}

		ml.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-ml.windowDuration)

		// Clean old attempts for this IP
		attempts := ml.attempts[clientIP]
		filtered := attempts[:0]
		for _, t := range attempts {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		ml.attempts[clientIP] = filtered

		// Check limit - 5 attempts per 5 minutes
		if len(filtered) >= ml.maxAttempts {
			ml.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "300")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"mfa_rate_limit","message":"Too many MFA attempts. Try again in 5 minutes."}`))
			return
		}

		// Add current attempt
		ml.attempts[clientIP] = append(ml.attempts[clientIP], now)
		ml.mu.Unlock()

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
					_, _ = w.Write([]byte(`{"error":"internal_error","message":"An unexpected error occurred"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders adds security headers including Content-Security-Policy.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy - restrict resources to same origin
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' wss: https:; frame-ancestors 'none'")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Enable XSS filter
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}
