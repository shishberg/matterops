package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shishberg/matterops/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type mockDeployTrigger struct {
	called     bool
	calledRepo string
	calledRef  string
}

func (m *mockDeployTrigger) HandlePush(repo string, branch string) {
	m.called = true
	m.calledRepo = repo
	m.calledRef = branch
}

func TestWebhook_ValidPush(t *testing.T) {
	const secret = "test-secret"
	trigger := &mockDeployTrigger{}
	handler := webhook.NewHandler(secret, trigger)

	payload := map[string]interface{}{
		"ref": "refs/heads/main",
		"repository": map[string]interface{}{
			"full_name": "acme/myapp",
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(body)))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, trigger.called)
	assert.Equal(t, "acme/myapp", trigger.calledRepo)
	assert.Equal(t, "main", trigger.calledRef)
}

func TestWebhook_InvalidSignature(t *testing.T) {
	const secret = "test-secret"
	trigger := &mockDeployTrigger{}
	handler := webhook.NewHandler(secret, trigger)

	payload := map[string]interface{}{
		"ref": "refs/heads/main",
		"repository": map[string]interface{}{
			"full_name": "acme/myapp",
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(body)))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidsignature")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.False(t, trigger.called)
}

func TestWebhook_NonPushEvent(t *testing.T) {
	const secret = "test-secret"
	trigger := &mockDeployTrigger{}
	handler := webhook.NewHandler(secret, trigger)

	payload := map[string]interface{}{
		"action": "opened",
		"pull_request": map[string]interface{}{
			"number": 42,
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(body)))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.False(t, trigger.called)
}

func TestWebhook_TagPushIgnored(t *testing.T) {
	const secret = "test-secret"
	trigger := &mockDeployTrigger{}
	handler := webhook.NewHandler(secret, trigger)

	payload := map[string]interface{}{
		"ref": "refs/tags/v1.0.0",
		"repository": map[string]interface{}{
			"full_name": "acme/myapp",
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(body)))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.False(t, trigger.called)
}
