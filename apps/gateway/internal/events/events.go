package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// EventType defines the type of event being published
type EventType string

const (
	EventTypeTaskCreated EventType = "task_created"
	EventTypeTaskUpdated EventType = "task_updated"
)

// Event represents the structure of an event message
type Event struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"project_id"`
	EventType   EventType   `json:"event_type"`
	Payload     interface{} `json:"payload"`
	PublishedAt string      `json:"published_at"`
}

// Service handles event publishing
type Service struct {
	redis *redis.Client
}

// New creates a new events service
func New(redisClient *redis.Client) *Service {
	return &Service{
		redis: redisClient,
	}
}

// Publish publishes an event to the shared Redis channel
func (s *Service) Publish(ctx context.Context, projectID string, eventType EventType, payload interface{}) error {
	event := Event{
		ID:          fmt.Sprintf("%s-%d", eventType, time.Now().UnixNano()), // Simple unique ID
		ProjectID:   projectID,
		EventType:   eventType,
		Payload:     payload,
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to the "kyros:events" channel, matching the Python service's subscription
	err = s.redis.Publish(ctx, "kyros:events", data).Err()
	if err != nil {
		return fmt.Errorf("failed to publish event to redis: %w", err)
	}

	return nil
}
