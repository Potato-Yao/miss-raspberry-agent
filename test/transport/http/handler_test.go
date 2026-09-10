package http_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	transporthttp "miss-raspberry-agent/internal/transport/http"
	"miss-raspberry-agent/internal/transport/http/handler"
)

func TestSendMessageAcceptedAndQueued(t *testing.T) {
	router, queue := newTestRouter()
	rec := doRequest(router, http.MethodPost, "/api/v1/agents/main/messages", "Bearer "+testToken, validBody())
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" || resp.Status != "queued" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	items := queue.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 queued item, got %d", len(items))
	}
	got := items[0]
	if got.ID != resp.ID {
		t.Errorf("queued id = %q, response id = %q", got.ID, resp.ID)
	}
	if got.Content != "你好" || got.Context != "初次打招呼" {
		t.Errorf("unexpected item: %+v", got)
	}
	if got.TargetType != "private" || got.TargetID != 10001 {
		t.Errorf("unexpected target: %+v", got)
	}
}

func TestSendMessageRejectsBadRequests(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"malformed json", `{"platform":`, http.StatusBadRequest},
		{"missing platform", `{"target_id":"1","content":"hi"}`, http.StatusBadRequest},
		{"missing content", `{"platform":"qq","target_id":"1"}`, http.StatusBadRequest},
		{"unsupported platform", `{"platform":"discord","target_id":"1","content":"hi"}`, http.StatusUnprocessableEntity},
		{"non-numeric target", `{"platform":"qq","target_id":"abc","content":"hi"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, queue := newTestRouter()
			rec := doRequest(router, http.MethodPost, "/api/v1/agents/main/messages", "Bearer "+testToken, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			if got := decodeError(t, rec); got == "" {
				t.Error("expected a non-empty error message")
			}
			if !queue.IsEmpty() {
				t.Error("rejected request must not be queued")
			}
		})
	}
}

func TestSendMessageInternalError(t *testing.T) {
	router := transporthttp.NewRouter(handler.NewMessageHandler(fakeSubmitter{err: errors.New("boom")}), testToken)
	rec := doRequest(router, http.MethodPost, "/api/v1/agents/main/messages", "Bearer "+testToken, validBody())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := decodeError(t, rec); got != "internal error" {
		t.Errorf("error = %q, want %q", got, "internal error")
	}
}
