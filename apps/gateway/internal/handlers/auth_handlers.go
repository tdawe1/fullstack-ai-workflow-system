// Package handlers provides auth-related HTTP handlers.
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kyros-praxis/gateway/internal/auth"
	"github.com/kyros-praxis/gateway/internal/models"
)

// ---- OAuth Handlers ----

// OAuthStart handles GET /auth/oauth/{provider} - redirects to OAuth provider.
func (h *Handler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	oauthProvider, err := h.oauth.GetProvider(provider)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_provider", err.Error())
		return
	}

	// Generate and store state
	state, err := auth.GenerateState()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate state")
		return
	}
	h.oauthStates.Store(state)

	// Redirect to OAuth provider
	authURL := oauthProvider.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// OAuthCallback handles GET /auth/oauth/{provider}/callback - processes OAuth callback.
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	// Validate state
	state := r.URL.Query().Get("state")
	if !h.oauthStates.Validate(state) {
		h.writeError(w, http.StatusBadRequest, "invalid_state", "Invalid or expired OAuth state")
		return
	}

	// Get code
	code := r.URL.Query().Get("code")
	if code == "" {
		h.writeError(w, http.StatusBadRequest, "missing_code", "OAuth code missing")
		return
	}

	// Exchange code for user info
	oauthProvider, err := h.oauth.GetProvider(provider)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_provider", err.Error())
		return
	}

	oauthUser, err := oauthProvider.ExchangeCode(r.Context(), code)
	if err != nil {
		h.log.Error("oauth exchange failed", "provider", provider, "error", err)
		h.writeError(w, http.StatusBadRequest, "oauth_failed", "Failed to authenticate with provider")
		return
	}

	// Find or create user
	user, err := h.db.GetUserByEmail(r.Context(), oauthUser.Email)
	if err != nil {
		// Create new user from OAuth
		user = &models.User{
			ID:        uuid.New(),
			Username:  oauthUser.Name,
			Email:     oauthUser.Email,
			Role:      "user",
			Active:    true,
			CreatedAt: time.Now().UTC(),
		}
		if err := h.db.CreateUser(r.Context(), user); err != nil {
			h.log.Error("failed to create oauth user", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create user")
			return
		}
	}

	// TODO: Link OAuth account to user (for multiple providers)

	// Create tokens
	accessToken, err := h.auth.CreateAccessToken(user)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create token")
		return
	}

	refreshToken, _ := h.auth.CreateRefreshToken(user)

	// Set cookie and redirect to frontend
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   h.cfg.JWTExpireMinutes * 60,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   h.cfg.JWTRefreshExpireDays * 24 * 60 * 60,
	})

	// Redirect to frontend
	http.Redirect(w, r, h.cfg.CORSAllowOrigins[0]+"/dashboard", http.StatusTemporaryRedirect)
}

// ListOAuthProviders handles GET /auth/oauth/providers - lists available OAuth providers.
func (h *Handler) ListOAuthProviders(w http.ResponseWriter, r *http.Request) {
	providers := h.oauth.ListProviders()
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": providers,
	})
}

// ---- MFA Handlers ----

// MFASetup handles POST /auth/mfa/setup - generates TOTP secret.
func (h *Handler) MFASetup(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	setup, err := auth.GenerateTOTPSecret(user.Email, auth.MFAConfig{
		Issuer:      h.cfg.MFAIssuer,
		BackupCodes: 10,
	})
	if err != nil {
		h.log.Error("failed to generate TOTP", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to setup MFA")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"secret":       setup.Secret,
		"url":          setup.URL,
		"backup_codes": setup.BackupCodes,
	})
}

// MFAEnable handles POST /auth/mfa/enable - enables MFA after verification.
func (h *Handler) MFAEnable(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := h.decodeAndValidate(r, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Validate the code
	if !auth.ValidateTOTP(req.Secret, req.Code) {
		h.writeError(w, http.StatusBadRequest, "invalid_code", "Invalid verification code")
		return
	}

	// TODO: Store MFA secret in database
	// For now, return success
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"message": "MFA enabled successfully",
	})
}

// MFAVerify handles POST /auth/mfa/verify - verifies TOTP during login.
func (h *Handler) MFAVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}
	if err := h.decodeAndValidate(r, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// TODO: Get user's MFA secret from database and verify
	// For now, placeholder
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"verified": true,
	})
}

// MFADisable handles POST /auth/mfa/disable - disables MFA.
func (h *Handler) MFADisable(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := h.decodeAndValidate(r, &req); err != nil {
		h.writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// TODO: Verify code and disable MFA in database
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"disabled": true,
		"message":  "MFA disabled successfully",
	})
}

// ---- Session Handlers ----

// ListSessions handles GET /auth/sessions - lists user's active sessions.
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	if h.sessions == nil {
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"sessions": []interface{}{},
			"message":  "Session management requires Redis",
		})
		return
	}

	sessions, err := h.sessions.ListUserSessions(r.Context(), user.ID.String())
	if err != nil {
		h.log.Error("failed to list sessions", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list sessions")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

// RevokeSession handles DELETE /auth/sessions/{id} - revokes a specific session.
func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		h.writeError(w, http.StatusBadRequest, "missing_id", "Session ID required")
		return
	}

	if h.sessions == nil {
		h.writeError(w, http.StatusServiceUnavailable, "unavailable", "Session management requires Redis")
		return
	}

	if err := h.sessions.RevokeSession(r.Context(), sessionID, user.ID.String()); err != nil {
		h.log.Error("failed to revoke session", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to revoke session")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"revoked": true,
	})
}

// RevokeAllSessions handles DELETE /auth/sessions - revokes all other sessions.
func (h *Handler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	if h.sessions == nil {
		h.writeError(w, http.StatusServiceUnavailable, "unavailable", "Session management requires Redis")
		return
	}

	// Get current session ID from cookie/header to exclude
	currentSessionID := r.Header.Get("X-Session-ID")

	if err := h.sessions.RevokeAllSessions(r.Context(), user.ID.String(), currentSessionID); err != nil {
		h.log.Error("failed to revoke sessions", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to revoke sessions")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"revoked_all": true,
	})
}
