// Package dto holds the project-owned HTTP request/response types. They are deliberately
// decoupled from internal application types so the API stays stable across refactors.
package dto

// SendMessageRequest is the body of POST /api/v1/agents/main/messages.
type SendMessageRequest struct {
	Platform string `json:"platform" binding:"required"`
	TargetID string `json:"target_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
	Context  string `json:"context,omitempty"`
}

// SendMessageResponse is returned once a message has been accepted into the agent's queue.
type SendMessageResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// HealthResponse is returned by GET /healthz.
type HealthResponse struct {
	Status string `json:"status"`
}

// ErrorResponse is the uniform error body for all API failures.
type ErrorResponse struct {
	Error string `json:"error"`
}
