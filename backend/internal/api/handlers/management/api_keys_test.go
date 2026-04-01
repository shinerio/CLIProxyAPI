package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestAPIKeysHandlersSupportCustomName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("api-keys: []\n"), 0644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	cfg := &config.Config{}
	handler := NewHandler(cfg, configPath, nil)

	performJSONRequest(t, handler.PutAPIKeys, http.MethodPut, `[{"name":"Primary","key":"sk-primary","allowed-auth-indices":["auth-b","auth-a","auth-a"]},"sk-backup"]`)

	if len(cfg.APIKeys) != 2 {
		t.Fatalf("expected 2 api keys after put, got %d", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0].Name != "Primary" || cfg.APIKeys[0].Key != "sk-primary" {
		t.Fatalf("unexpected primary entry: %#v", cfg.APIKeys[0])
	}
	if len(cfg.APIKeys[0].AllowedAuthIndices) != 2 ||
		cfg.APIKeys[0].AllowedAuthIndices[0] != "auth-a" ||
		cfg.APIKeys[0].AllowedAuthIndices[1] != "auth-b" {
		t.Fatalf("unexpected allowed auth indices: %v", cfg.APIKeys[0].AllowedAuthIndices)
	}

	performJSONRequest(t, handler.PatchAPIKeys, http.MethodPatch, `{"old":null,"new":{"name":"Fallback","key":"sk-fallback"}}`)
	if len(cfg.APIKeys) != 3 {
		t.Fatalf("expected 3 api keys after append patch, got %d", len(cfg.APIKeys))
	}
	if cfg.APIKeys[2].Name != "Fallback" || cfg.APIKeys[2].Key != "sk-fallback" {
		t.Fatalf("unexpected appended entry: %#v", cfg.APIKeys[2])
	}

	recorder := performRequest(t, handler.GetAPIKeys, http.MethodGet, "", "")
	var payload struct {
		APIKeys []any `json:"api-keys"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if len(payload.APIKeys) != 3 {
		t.Fatalf("expected 3 api keys in response, got %d", len(payload.APIKeys))
	}
	first, ok := payload.APIKeys[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first entry to be object, got %#v", payload.APIKeys[0])
	}
	if first["name"] != "Primary" || first["key"] != "sk-primary" {
		t.Fatalf("unexpected first response entry: %#v", first)
	}
	if _, ok := payload.APIKeys[1].(string); !ok {
		t.Fatalf("expected second entry to remain legacy string, got %#v", payload.APIKeys[1])
	}
}

func performJSONRequest(t *testing.T, handlerFunc func(*gin.Context), method, body string) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, handlerFunc, method, body, "application/json")
}

func performRequest(t *testing.T, handlerFunc func(*gin.Context), method, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, "/v0/management/api-keys", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	ctx.Request = req

	handlerFunc(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	return recorder
}
