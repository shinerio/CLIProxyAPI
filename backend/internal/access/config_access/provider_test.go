package configaccess

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestProviderAuthenticate_ExposesAllowedAuthIndicesMetadata(t *testing.T) {
	t.Parallel()

	entries := normalizeEntries([]internalconfig.ClientAPIKeyConfig{
		{
			Key:                "sk-test",
			AllowedAuthIndices: []string{"zeta", " alpha ", "", "beta", "alpha"},
		},
	})
	provider := newProvider("config-test", entries)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-test")

	result, authErr := provider.Authenticate(context.Background(), req)
	if authErr != nil {
		t.Fatalf("Authenticate() authErr = %v", authErr)
	}
	if result == nil {
		t.Fatal("Authenticate() result = nil")
	}
	if result.Provider != "config-test" {
		t.Fatalf("Authenticate() provider = %q, want %q", result.Provider, "config-test")
	}
	if result.Principal != "sk-test" {
		t.Fatalf("Authenticate() principal = %q, want %q", result.Principal, "sk-test")
	}
	if got := result.Metadata["source"]; got != "authorization" {
		t.Fatalf("Authenticate() metadata[source] = %q, want %q", got, "authorization")
	}
	if got := result.Metadata["allowed_auth_indices"]; got != "alpha,beta,zeta" {
		t.Fatalf("Authenticate() metadata[allowed_auth_indices] = %q, want %q", got, "alpha,beta,zeta")
	}
}
