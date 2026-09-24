// Package handler translates HTTP requests into application-service calls. It contains no
// business logic: validation beyond HTTP shape and error-to-status mapping lives in the service.
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"miss-raspberry-agent/internal/adapter/http/dto"
	"miss-raspberry-agent/internal/messaging"
)

// Submitter is the application service the message handler depends on.
type Submitter interface {
	Submit(ctx context.Context, msg messaging.Message) (string, error)
}

// MessageHandler serves the message-submission endpoint.
type MessageHandler struct {
	submitter Submitter
}

// NewMessageHandler creates a MessageHandler backed by the given submitter.
func NewMessageHandler(submitter Submitter) *MessageHandler {
	return &MessageHandler{submitter: submitter}
}

// Send handles POST /api/v1/agents/main/messages: it validates the JSON body, submits the
// message to the agent's queue, and returns 202 Accepted with the created todo id.
func (h *MessageHandler) Send(c *gin.Context) {
	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	id, err := h.submitter.Submit(c.Request.Context(), messaging.Message{
		Platform: req.Platform,
		TargetID: req.TargetID,
		Content:  req.Content,
		Context:  req.Context,
	})
	if err != nil {
		switch {
		case errors.Is(err, messaging.ErrUnsupportedPlatform):
			c.JSON(http.StatusUnprocessableEntity, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, messaging.ErrInvalidMessage):
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		default:
			_ = c.Error(err)
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal error"})
		}
		return
	}

	c.JSON(http.StatusAccepted, dto.SendMessageResponse{ID: id, Status: "queued"})
}
