// Package auth provides Redis-backed session management.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Session represents an active user session.
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	DeviceInfo string    `json:"device_info"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// SessionManager manages user sessions in Redis.
type SessionManager struct {
	client     *redis.Client
	sessionTTL time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager(redisURL string, sessionTTL time.Duration) (*SessionManager, error) {
	if redisURL == "" {
		return nil, nil // Sessions disabled if no Redis
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &SessionManager{
		client:     client,
		sessionTTL: sessionTTL,
	}, nil
}

// Close closes the Redis connection.
func (m *SessionManager) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// sessionKey returns the Redis key for a session.
func sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// userSessionsKey returns the Redis key for a user's session list.
func userSessionsKey(userID string) string {
	return fmt.Sprintf("user_sessions:%s", userID)
}

// CreateSession creates a new session for a user.
func (m *SessionManager) CreateSession(ctx context.Context, userID, deviceInfo, ipAddress, userAgent string) (*Session, error) {
	if m == nil {
		return nil, nil
	}

	session := &Session{
		ID:         uuid.New().String(),
		UserID:     userID,
		DeviceInfo: deviceInfo,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		CreatedAt:  time.Now().UTC(),
		LastActive: time.Now().UTC(),
		ExpiresAt:  time.Now().UTC().Add(m.sessionTTL),
	}

	data, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	pipe := m.client.Pipeline()

	// Store session
	pipe.Set(ctx, sessionKey(session.ID), data, m.sessionTTL)

	// Add to user's session set
	pipe.SAdd(ctx, userSessionsKey(userID), session.ID)
	pipe.Expire(ctx, userSessionsKey(userID), m.sessionTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (m *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if m == nil {
		return nil, nil
	}

	data, err := m.client.Get(ctx, sessionKey(sessionID)).Bytes()
	if err == redis.Nil {
		return nil, nil // Session not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// UpdateLastActive updates the last active time of a session.
func (m *SessionManager) UpdateLastActive(ctx context.Context, sessionID string) error {
	if m == nil {
		return nil
	}

	session, err := m.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return err
	}

	session.LastActive = time.Now().UTC()

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Get remaining TTL and update
	ttl, err := m.client.TTL(ctx, sessionKey(sessionID)).Result()
	if err != nil {
		return fmt.Errorf("failed to get TTL: %w", err)
	}

	return m.client.Set(ctx, sessionKey(sessionID), data, ttl).Err()
}

// ListUserSessions lists all active sessions for a user.
func (m *SessionManager) ListUserSessions(ctx context.Context, userID string) ([]Session, error) {
	if m == nil {
		return nil, nil
	}

	sessionIDs, err := m.client.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session IDs: %w", err)
	}

	sessions := make([]Session, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, err := m.GetSession(ctx, id)
		if err != nil {
			continue // Skip errored sessions
		}
		if session != nil {
			sessions = append(sessions, *session)
		} else {
			// Clean up stale reference
			m.client.SRem(ctx, userSessionsKey(userID), id)
		}
	}

	return sessions, nil
}

// RevokeSession revokes a specific session.
func (m *SessionManager) RevokeSession(ctx context.Context, sessionID, userID string) error {
	if m == nil {
		return nil
	}

	pipe := m.client.Pipeline()
	pipe.Del(ctx, sessionKey(sessionID))
	pipe.SRem(ctx, userSessionsKey(userID), sessionID)

	_, err := pipe.Exec(ctx)
	return err
}

// RevokeAllSessions revokes all sessions for a user except the current one.
func (m *SessionManager) RevokeAllSessions(ctx context.Context, userID, exceptSessionID string) error {
	if m == nil {
		return nil
	}

	sessionIDs, err := m.client.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil {
		return fmt.Errorf("failed to get session IDs: %w", err)
	}

	pipe := m.client.Pipeline()
	for _, id := range sessionIDs {
		if id != exceptSessionID {
			pipe.Del(ctx, sessionKey(id))
			pipe.SRem(ctx, userSessionsKey(userID), id)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

// RevokeAllUserSessions revokes ALL sessions for a user (used on password change).
func (m *SessionManager) RevokeAllUserSessions(ctx context.Context, userID string) error {
	if m == nil {
		return nil
	}

	sessionIDs, err := m.client.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil {
		return fmt.Errorf("failed to get session IDs: %w", err)
	}

	pipe := m.client.Pipeline()
	for _, id := range sessionIDs {
		pipe.Del(ctx, sessionKey(id))
	}
	pipe.Del(ctx, userSessionsKey(userID))

	_, err = pipe.Exec(ctx)
	return err
}
