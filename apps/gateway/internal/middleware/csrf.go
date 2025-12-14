// Package middleware provides CSRF protection.
package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

// CSRFConfig holds CSRF configuration.
type CSRFConfig struct {
	TokenLength   int
	CookieName    string
	HeaderName    string
	CookieSecure  bool
	CookiePath    string
	TokenLifetime time.Duration
}

// DefaultCSRFConfig returns default CSRF configuration.
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		TokenLength:   32,
		CookieName:    "csrf_token",
		HeaderName:    "X-CSRF-Token",
		CookieSecure:  true,
		CookiePath:    "/",
		TokenLifetime: time.Hour,
	}
}

// CSRFProtection provides CSRF token generation and validation.
type CSRFProtection struct {
	config CSRFConfig
	tokens map[string]time.Time
	mu     sync.RWMutex
}

// NewCSRFProtection creates a new CSRF protection middleware.
func NewCSRFProtection(cfg CSRFConfig) *CSRFProtection {
	csrf := &CSRFProtection{
		config: cfg,
		tokens: make(map[string]time.Time),
	}
	// Start cleanup goroutine
	go csrf.cleanupLoop()
	return csrf
}

// cleanupLoop removes expired tokens periodically.
func (c *CSRFProtection) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for token, expiry := range c.tokens {
			if now.After(expiry) {
				delete(c.tokens, token)
			}
		}
		c.mu.Unlock()
	}
}

// GenerateToken generates a new CSRF token.
func (c *CSRFProtection) GenerateToken() (string, error) {
	bytes := make([]byte, c.config.TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(bytes)

	c.mu.Lock()
	c.tokens[token] = time.Now().Add(c.config.TokenLifetime)
	c.mu.Unlock()

	return token, nil
}

// ValidateToken validates a CSRF token.
func (c *CSRFProtection) ValidateToken(token string) bool {
	c.mu.RLock()
	expiry, exists := c.tokens[token]
	c.mu.RUnlock()

	if !exists {
		return false
	}
	if time.Now().After(expiry) {
		return false
	}
	return true
}

// Middleware returns CSRF protection middleware.
// Protects state-changing methods (POST, PUT, DELETE, PATCH).
// Safe methods (GET, HEAD, OPTIONS) get a token set in cookie.
func (c *CSRFProtection) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for API routes with Authorization header (API clients)
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}

		// Safe methods - set token in cookie
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			// Check if token already exists in cookie
			if _, err := r.Cookie(c.config.CookieName); err != nil {
				token, _ := c.GenerateToken()
				http.SetCookie(w, &http.Cookie{
					Name:     c.config.CookieName,
					Value:    token,
					Path:     c.config.CookiePath,
					Secure:   c.config.CookieSecure,
					HttpOnly: false, // Needs to be readable by JS
					SameSite: http.SameSiteStrictMode,
				})
			}
			next.ServeHTTP(w, r)
			return
		}

		// State-changing methods - validate token
		headerToken := r.Header.Get(c.config.HeaderName)
		cookieToken, err := r.Cookie(c.config.CookieName)

		if err != nil || headerToken == "" {
			http.Error(w, `{"error":"csrf_token_missing","message":"CSRF token required"}`, http.StatusForbidden)
			return
		}

		// Both tokens must match and be valid
		if headerToken != cookieToken.Value || !c.ValidateToken(headerToken) {
			http.Error(w, `{"error":"csrf_token_invalid","message":"Invalid CSRF token"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
