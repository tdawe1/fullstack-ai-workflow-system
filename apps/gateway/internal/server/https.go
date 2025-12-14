// Package server provides HTTP/HTTPS server utilities.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// Config holds server configuration.
type Config struct {
	Port        string
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string
	TLSAutoLets bool
	TLSDomain   string
}

// Server wraps http.Server with TLS support.
type Server struct {
	httpServer *http.Server
	config     Config
	log        *slog.Logger
}

// New creates a new server with the given handler.
func New(handler http.Handler, cfg Config, log *slog.Logger) *Server {
	addr := ":" + cfg.Port
	if cfg.TLSEnabled && cfg.Port == "8001" {
		// Default to 443 for HTTPS if using default port
		addr = ":443"
	}

	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		config: cfg,
		log:    log,
	}
}

// Start starts the server (HTTP or HTTPS based on config).
func (s *Server) Start() error {
	if s.config.TLSEnabled {
		return s.startHTTPS()
	}
	return s.startHTTP()
}

// startHTTP starts a plain HTTP server.
func (s *Server) startHTTP() error {
	s.log.Info("Starting HTTP server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// startHTTPS starts an HTTPS server.
func (s *Server) startHTTPS() error {
	if s.config.TLSAutoLets {
		return s.startWithAutoTLS()
	}
	return s.startWithCertFiles()
}

// startWithCertFiles starts HTTPS with provided certificate files.
func (s *Server) startWithCertFiles() error {
	if s.config.TLSCertFile == "" || s.config.TLSKeyFile == "" {
		return fmt.Errorf("TLS enabled but TLS_CERT_FILE and TLS_KEY_FILE not set")
	}

	// Configure TLS with modern settings
	s.httpServer.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}

	s.log.Info("Starting HTTPS server with certificate files",
		"addr", s.httpServer.Addr,
		"cert", s.config.TLSCertFile,
	)

	return s.httpServer.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
}

// startWithAutoTLS starts HTTPS with Let's Encrypt auto-renewal.
func (s *Server) startWithAutoTLS() error {
	if s.config.TLSDomain == "" {
		return fmt.Errorf("TLS_AUTO_LETSENCRYPT enabled but TLS_DOMAIN not set")
	}

	// Create autocert manager
	certManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(s.config.TLSDomain),
		Cache:      autocert.DirCache("./certs"), // Store certs in ./certs
	}

	// Configure server for autocert
	s.httpServer.TLSConfig = certManager.TLSConfig()
	s.httpServer.TLSConfig.MinVersion = tls.VersionTLS12

	s.log.Info("Starting HTTPS server with Let's Encrypt",
		"addr", s.httpServer.Addr,
		"domain", s.config.TLSDomain,
	)

	// Start HTTP server on port 80 for ACME challenges
	go func() {
		httpServer := &http.Server{
			Addr:    ":80",
			Handler: certManager.HTTPHandler(nil),
		}
		s.log.Info("Starting HTTP server for ACME challenges", "addr", ":80")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("ACME HTTP server error", "error", err)
		}
	}()

	return s.httpServer.ListenAndServeTLS("", "")
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// HTTPSRedirectMiddleware redirects HTTP to HTTPS in production.
func HTTPSRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request is HTTP (not HTTPS)
		if r.Header.Get("X-Forwarded-Proto") == "http" {
			target := "https://" + r.Host + r.URL.Path
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware adds security headers for HTTPS.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HSTS - enforce HTTPS for 1 year
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")
		// Prevent MIME sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// XSS Protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Content Security Policy (basic)
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		next.ServeHTTP(w, r)
	})
}
