// Package http wires the HTTP transport: routes, middleware, and the server lifecycle. Gin
// types stay inside this package tree.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"miss-raspberry-agent/internal/transport/http/dto"
	"miss-raspberry-agent/internal/transport/http/handler"
	"miss-raspberry-agent/internal/transport/http/middleware"
)

// NewRouter builds the HTTP router. The health endpoint is public; everything under /api/v1
// requires the bearer token.
func NewRouter(messageHandler *handler.MessageHandler, apiToken string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, dto.HealthResponse{Status: "ok"})
	})

	api := router.Group("/api/v1")
	api.Use(middleware.TokenAuth(apiToken))
	api.POST("/agents/main/messages", messageHandler.Send)

	return router
}
