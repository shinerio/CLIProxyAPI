package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClientAPIKeyConfigJSONCompat(t *testing.T) {
	t.Run("legacy string", func(t *testing.T) {
		var entry ClientAPIKeyConfig
		if err := json.Unmarshal([]byte(`"sk-test"`), &entry); err != nil {
			t.Fatalf("unmarshal legacy string: %v", err)
		}
		if entry.Name != "" {
			t.Fatalf("expected empty name, got %q", entry.Name)
		}
		if entry.Key != "sk-test" {
			t.Fatalf("expected key sk-test, got %q", entry.Key)
		}
		if len(entry.AllowedAuthIndices) != 0 {
			t.Fatalf("expected no allowed auth indices, got %v", entry.AllowedAuthIndices)
		}
	})

	t.Run("named object marshals as object", func(t *testing.T) {
		data, err := json.Marshal(ClientAPIKeyConfig{
			Name:               "Primary",
			Key:                "sk-test",
			AllowedAuthIndices: []string{"b", "a", "a"},
		})
		if err != nil {
			t.Fatalf("marshal named object: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("decode marshaled payload: %v", err)
		}
		if got := payload["name"]; got != "Primary" {
			t.Fatalf("expected name Primary, got %#v", got)
		}
		if got := payload["key"]; got != "sk-test" {
			t.Fatalf("expected key sk-test, got %#v", got)
		}
		rawAllowed, ok := payload["allowed-auth-indices"].([]any)
		if !ok || len(rawAllowed) != 2 || rawAllowed[0] != "a" || rawAllowed[1] != "b" {
			t.Fatalf("expected normalized allowed auth indices [a b], got %#v", payload["allowed-auth-indices"])
		}
	})
}

func TestClientAPIKeyConfigYAMLCompat(t *testing.T) {
	const input = `
api-keys:
  - legacy-key
  - name: Team A
    key: sk-team-a
    allowed-auth-indices:
      - auth-b
      - auth-a
      - auth-a
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal yaml: %v", err)
	}
	if len(cfg.APIKeys) != 2 {
		t.Fatalf("expected 2 api keys, got %d", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0].Key != "legacy-key" || cfg.APIKeys[0].Name != "" {
		t.Fatalf("unexpected legacy entry: %#v", cfg.APIKeys[0])
	}
	if cfg.APIKeys[1].Name != "Team A" {
		t.Fatalf("expected name Team A, got %q", cfg.APIKeys[1].Name)
	}
	if len(cfg.APIKeys[1].AllowedAuthIndices) != 2 ||
		cfg.APIKeys[1].AllowedAuthIndices[0] != "auth-a" ||
		cfg.APIKeys[1].AllowedAuthIndices[1] != "auth-b" {
		t.Fatalf("unexpected allowed auth indices: %v", cfg.APIKeys[1].AllowedAuthIndices)
	}

	output, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	if string(output) == "" {
		t.Fatal("expected marshaled yaml output")
	}
	if !containsAll(string(output), "legacy-key", "name: Team A", "key: sk-team-a", "allowed-auth-indices:") {
		t.Fatalf("unexpected yaml output:\n%s", string(output))
	}
}

func containsAll(s string, values ...string) bool {
	for _, value := range values {
		if !strings.Contains(s, value) {
			return false
		}
	}
	return true
}
