package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorLogger logs server-side failures (status >= 500) once the response has been written.
// Handlers attach the underlying cause with c.Error(err); the response body itself stays
// generic so internals are not exposed to clients.
//
// Register it outside the recovery middleware, e.g.
// router.Use(gin.Logger(), ErrorLogger(), gin.Recovery()), so a recovered panic is observed
// here as a 500 as well. For hard panics gin.Recovery still prints the stack trace.
func ErrorLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Log every 5xx (status >= 500) response. This covers handler-produced
		// internal errors, recovered panics, and dependency/framework 5xx alike.
		if status := c.Writer.Status(); status >= http.StatusInternalServerError {
			if len(c.Errors) > 0 {
				log.Printf("[http] %s %s -> %d: %s", c.Request.Method, c.Request.URL.Path, status, c.Errors.String())
				return
			}
			log.Printf("[http] %s %s -> %d", c.Request.Method, c.Request.URL.Path, status)
		}
	}
}
