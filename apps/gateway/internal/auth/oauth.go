// Package auth provides OAuth 2.0 authentication with multiple providers.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthUser represents a user returned from an OAuth provider.
type OAuthUser struct {
	ProviderID   string `json:"provider_id"`
	Provider     string `json:"provider"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	AvatarURL    string `json:"avatar_url"`
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
}

// OAuthProvider defines the interface for OAuth providers.
type OAuthProvider interface {
	Name() string
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*OAuthUser, error)
}

// OAuthConfig holds OAuth provider configurations.
type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string
}

// OAuthManager manages multiple OAuth providers.
type OAuthManager struct {
	providers map[string]OAuthProvider
}

// NewOAuthManager creates a new OAuth manager with configured providers.
func NewOAuthManager(cfg OAuthConfig) *OAuthManager {
	m := &OAuthManager{
		providers: make(map[string]OAuthProvider),
	}

	// Register Google if configured
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		m.providers["google"] = &GoogleProvider{
			config: &oauth2.Config{
				ClientID:     cfg.GoogleClientID,
				ClientSecret: cfg.GoogleClientSecret,
				RedirectURL:  cfg.GoogleRedirectURL,
				Scopes:       []string{"openid", "email", "profile"},
				Endpoint:     google.Endpoint,
			},
		}
	}

	// Register GitHub if configured
	if cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "" {
		m.providers["github"] = &GitHubProvider{
			config: &oauth2.Config{
				ClientID:     cfg.GitHubClientID,
				ClientSecret: cfg.GitHubClientSecret,
				RedirectURL:  cfg.GitHubRedirectURL,
				Scopes:       []string{"user:email", "read:user"},
				Endpoint:     github.Endpoint,
			},
		}
	}

	return m
}

// GetProvider returns an OAuth provider by name.
func (m *OAuthManager) GetProvider(name string) (OAuthProvider, error) {
	p, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("oauth provider '%s' not configured", name)
	}
	return p, nil
}

// ListProviders returns the names of all configured providers.
func (m *OAuthManager) ListProviders() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// GenerateState generates a random state string for OAuth.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ---- Google Provider ----

// GoogleProvider implements OAuth for Google.
type GoogleProvider struct {
	config *oauth2.Config
}

func (p *GoogleProvider) Name() string {
	return "google"
}

func (p *GoogleProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUser, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Fetch user info
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var info struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &OAuthUser{
		ProviderID:   info.ID,
		Provider:     "google",
		Email:        info.Email,
		Name:         info.Name,
		AvatarURL:    info.Picture,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	}, nil
}

// ---- GitHub Provider ----

// GitHubProvider implements OAuth for GitHub.
type GitHubProvider struct {
	config *oauth2.Config
}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUser, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	client := p.config.Client(ctx, token)

	// Fetch user info
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var info struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	// Fetch primary email
	email, err := p.fetchPrimaryEmail(client)
	if err != nil {
		return nil, err
	}

	name := info.Name
	if name == "" {
		name = info.Login
	}

	return &OAuthUser{
		ProviderID:   fmt.Sprintf("%d", info.ID),
		Provider:     "github",
		Email:        email,
		Name:         name,
		AvatarURL:    info.AvatarURL,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	}, nil
}

func (p *GitHubProvider) fetchPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", fmt.Errorf("failed to get emails: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read emails: %w", err)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("failed to parse emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	return "", errors.New("no verified primary email found")
}

// ---- State Store ----

// OAuthStateStore stores OAuth state tokens temporarily.
type OAuthStateStore struct {
	states map[string]time.Time
}

// NewOAuthStateStore creates a new state store.
func NewOAuthStateStore() *OAuthStateStore {
	return &OAuthStateStore{
		states: make(map[string]time.Time),
	}
}

// Store saves a state token.
func (s *OAuthStateStore) Store(state string) {
	s.states[state] = time.Now().Add(10 * time.Minute)
}

// Validate checks and removes a state token.
func (s *OAuthStateStore) Validate(state string) bool {
	exp, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Now().Before(exp)
}
