package http_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	transporthttp "miss-raspberry-agent/internal/adapter/http"
	"miss-raspberry-agent/internal/adapter/http/handler"
	"miss-raspberry-agent/internal/health"
	"miss-raspberry-agent/internal/tagging"
)

// captureLogs redirects the standard logger to a buffer for the duration of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func TestServerErrorIsLoggedWithCause(t *testing.T) {
	logs := captureLogs(t)

	router := transporthttp.NewRouter(
		handler.NewMessageHandler(fakeSubmitter{err: errors.New("boom")}),
		handler.NewTaggingHandler(tagging.NewService(tagging.NewStore(), &fakeTagger{})),
		handler.NewHealthHandler(health.NewService(nil)),
		testToken,
	)
	rec := doRequest(router, http.MethodPost, "/api/v1/agents/main/messages", "Bearer "+testToken, validBody())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	out := logs.String()
	if !strings.Contains(out, "POST /api/v1/agents/main/messages") {
		t.Errorf("log missing request context: %s", out)
	}
	if !strings.Contains(out, "500") {
		t.Errorf("log missing status: %s", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("log missing underlying error: %s", out)
	}
}

func TestClientErrorIsNotLogged(t *testing.T) {
	logs := captureLogs(t)

	router, _ := newTestRouter()
	rec := doRequest(router, http.MethodPost, "/api/v1/agents/main/messages", "Bearer "+testToken, `{"platform":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if out := logs.String(); strings.Contains(out, "[http]") {
		t.Errorf("4xx response should not be logged, got: %s", out)
	}
}

func TestDegradedReadinessIsLogged(t *testing.T) {
	logs := captureLogs(t)

	router := newHealthTestRouter(func(context.Context) error {
		return errors.New("no bot connected")
	})
	rec := doRequest(router, http.MethodGet, "/api/v1/health", "Bearer "+testToken, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	out := logs.String()
	if !strings.Contains(out, "GET /api/v1/health") || !strings.Contains(out, "503") {
		t.Errorf("expected 503 readiness to be logged, got: %s", out)
	}
}

// TestAnyServerErrorStatusIsLogged proves the logger covers the whole 5xx range,
// not only 500: a 502 added directly to the router must also be logged.
func TestAnyServerErrorStatusIsLogged(t *testing.T) {
	logs := captureLogs(t)

	router, _ := newTestRouter()
	router.GET("/boom", func(c *gin.Context) {
		c.JSON(http.StatusBadGateway, gin.H{"error": "bad gateway"})
	})
	rec := doRequest(router, http.MethodGet, "/boom", "", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	out := logs.String()
	if !strings.Contains(out, "GET /boom") || !strings.Contains(out, "502") {
		t.Errorf("expected 502 to be logged, got: %s", out)
	}
}
