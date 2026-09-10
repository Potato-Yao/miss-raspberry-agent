// Package middleware holds HTTP middleware shared across routes.
package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"miss-raspberry-agent/internal/transport/http/dto"
)

// TokenAuth returns middleware that accepts only requests carrying
// "Authorization: Bearer <token>". The comparison is constant-time to avoid leaking the token
// through timing.
func TokenAuth(token string) gin.HandlerFunc {
	expected := []byte("Bearer " + token)
	return func(c *gin.Context) {
		got := []byte(c.GetHeader("Authorization"))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized"})
			return
		}
		c.Next()
	}
}
