package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func doRequest(router *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(w, req)
	return w
}

func extractToken(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("extractToken: invalid JSON: %v — body: %s", err, w.Body.String())
	}
	token := resp["data"]["token"]
	if token == "" {
		t.Fatalf("extractToken: empty token in response: %s", w.Body.String())
	}
	return token
}
