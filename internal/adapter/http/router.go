// Package http wires the HTTP transport: routes, middleware, and the server lifecycle. Gin
// types stay inside this package tree.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"miss-raspberry-agent/internal/adapter/http/dto"
	"miss-raspberry-agent/internal/adapter/http/handler"
	"miss-raspberry-agent/internal/adapter/http/middleware"
)

// NewRouter builds the HTTP router. GET /healthz is public; everything under /api/v1 requires
// the bearer token.
func NewRouter(messageHandler *handler.MessageHandler, taggingHandler *handler.TaggingHandler, healthHandler *handler.HealthHandler, apiToken string) *gin.Engine {
	router := gin.New()
	// ErrorLogger must wrap Recovery so a panic that becomes a 500 is logged with request
	// context; Recovery still prints the stack trace.
	router.Use(gin.Logger(), middleware.ErrorLogger(), gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, dto.HealthResponse{Status: "ok"})
	})

	api := router.Group("/api/v1")
	api.Use(middleware.TokenAuth(apiToken))
	api.GET("/health", healthHandler.Check)
	api.POST("/agents/main/messages", messageHandler.Send)
	api.POST("/agents/tagger/tag-sets", taggingHandler.RegisterSet)
	api.POST("/agents/tagger/tag", taggingHandler.Tag)

	return router
}
