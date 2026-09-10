// Package message is the application service that accepts externally submitted messages and
// enqueues them for the main agent to handle. It owns the domain rules (supported platforms,
// required fields) so HTTP handlers only deal with transport concerns.
package message

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"miss-raspberry-agent/internal/tools/todo_list"
)

// PlatformQQ is the only message platform currently supported.
const PlatformQQ = "qq"

// ErrUnsupportedPlatform is returned when the requested platform is not supported.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// ErrInvalidMessage is returned when a required field is missing or malformed.
var ErrInvalidMessage = errors.New("invalid message")

// Message is a project-owned request to have the main agent produce and send a message.
// TargetID is a string so future platforms may use non-numeric identifiers; the QQ platform
// parses it into a numeric QQ id.
type Message struct {
	Platform string
	TargetID string
	Content  string
	Context  string
}

// Service validates submitted messages and pushes them into the main agent's todo queue.
type Service struct {
	queue *todo_list.Store
}

// NewService creates a message service backed by the given todo queue.
func NewService(queue *todo_list.Store) *Service {
	return &Service{queue: queue}
}

// Submit validates the message and enqueues it, returning the created todo item id. The agent
// loop picks the item up asynchronously.
func (s *Service) Submit(ctx context.Context, msg Message) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if msg.Platform != PlatformQQ {
		return "", fmt.Errorf("%w: %q (only %q is supported)", ErrUnsupportedPlatform, msg.Platform, PlatformQQ)
	}
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return "", fmt.Errorf("%w: content is required", ErrInvalidMessage)
	}
	targetID := strings.TrimSpace(msg.TargetID)
	if targetID == "" {
		return "", fmt.Errorf("%w: target_id is required", ErrInvalidMessage)
	}
	// The QQ platform addresses numeric QQ ids; other platforms may use arbitrary strings.
	qqID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil || qqID <= 0 {
		return "", fmt.Errorf("%w: target_id %q must be a positive QQ id for platform %q", ErrInvalidMessage, targetID, PlatformQQ)
	}
	if s.queue == nil {
		return "", errors.New("message: no todo queue configured")
	}

	item := s.queue.Add(todo_list.Item{
		Content:    content,
		Context:    strings.TrimSpace(msg.Context),
		Source:     fmt.Sprintf("API(平台=%s,目标=%s)", msg.Platform, targetID),
		TargetType: "private",
		TargetID:   qqID,
		UserID:     qqID,
	})
	return item.ID, nil
}
