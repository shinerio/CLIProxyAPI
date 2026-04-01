// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ClientAPIKeyConfig describes one incoming client API key and optional auth restrictions.
// When AllowedAuthIndices is empty, the key may use all available upstream credentials.
type ClientAPIKeyConfig struct {
	Name               string   `yaml:"name,omitempty" json:"name,omitempty"`
	Key                string   `yaml:"key,omitempty" json:"key,omitempty"`
	AllowedAuthIndices []string `yaml:"allowed-auth-indices,omitempty" json:"allowed-auth-indices,omitempty"`
}

type clientAPIKeyConfigAlias struct {
	Name               string   `yaml:"name" json:"name"`
	Key                string   `yaml:"key" json:"key"`
	APIKey             string   `yaml:"api-key" json:"api-key"`
	Value              string   `yaml:"value" json:"value"`
	AllowedAuthIndices []string `yaml:"allowed-auth-indices" json:"allowed-auth-indices"`
	AllowedAuths       []string `yaml:"allowed-auths" json:"allowed-auths"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// NormalizeAuthIndexList trims, deduplicates, and sorts auth_index lists.
func NormalizeAuthIndexList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

func (c *ClientAPIKeyConfig) normalize() {
	if c == nil {
		return
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Key = strings.TrimSpace(c.Key)
	c.AllowedAuthIndices = NormalizeAuthIndexList(c.AllowedAuthIndices)
}

// UnmarshalYAML supports both legacy string items and structured objects.
func (c *ClientAPIKeyConfig) UnmarshalYAML(value *yaml.Node) error {
	if c == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		var key string
		if err := value.Decode(&key); err != nil {
			return err
		}
		c.Name = ""
		c.Key = strings.TrimSpace(key)
		c.AllowedAuthIndices = nil
		return nil
	case yaml.MappingNode:
		var raw clientAPIKeyConfigAlias
		if err := value.Decode(&raw); err != nil {
			return err
		}
		c.Name = strings.TrimSpace(raw.Name)
		c.Key = strings.TrimSpace(firstNonEmpty(raw.Key, raw.APIKey, raw.Value))
		c.AllowedAuthIndices = NormalizeAuthIndexList(append(raw.AllowedAuthIndices, raw.AllowedAuths...))
		c.normalize()
		return nil
	default:
		return fmt.Errorf("invalid api-keys entry")
	}
}

// MarshalYAML emits the legacy scalar form when no restrictions are configured.
func (c ClientAPIKeyConfig) MarshalYAML() (any, error) {
	c.normalize()
	if c.Key == "" {
		return nil, nil
	}
	if c.Name == "" && len(c.AllowedAuthIndices) == 0 {
		return c.Key, nil
	}
	out := map[string]any{
		"key": c.Key,
	}
	if c.Name != "" {
		out["name"] = c.Name
	}
	if len(c.AllowedAuthIndices) > 0 {
		out["allowed-auth-indices"] = append([]string(nil), c.AllowedAuthIndices...)
	}
	return out, nil
}

// UnmarshalJSON supports both legacy string items and structured objects.
func (c *ClientAPIKeyConfig) UnmarshalJSON(data []byte) error {
	if c == nil {
		return nil
	}
	var key string
	if err := json.Unmarshal(data, &key); err == nil {
		c.Name = ""
		c.Key = strings.TrimSpace(key)
		c.AllowedAuthIndices = nil
		return nil
	}

	var raw clientAPIKeyConfigAlias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Name = strings.TrimSpace(raw.Name)
	c.Key = strings.TrimSpace(firstNonEmpty(raw.Key, raw.APIKey, raw.Value))
	c.AllowedAuthIndices = NormalizeAuthIndexList(append(raw.AllowedAuthIndices, raw.AllowedAuths...))
	c.normalize()
	return nil
}

// MarshalJSON emits the legacy string form when no restrictions are configured.
func (c ClientAPIKeyConfig) MarshalJSON() ([]byte, error) {
	c.normalize()
	if c.Name == "" && len(c.AllowedAuthIndices) == 0 {
		return json.Marshal(c.Key)
	}
	return json.Marshal(struct {
		Name               string   `json:"name,omitempty"`
		Key                string   `json:"key"`
		AllowedAuthIndices []string `json:"allowed-auth-indices,omitempty"`
	}{
		Name:               c.Name,
		Key:                c.Key,
		AllowedAuthIndices: append([]string(nil), c.AllowedAuthIndices...),
	})
}

// SDKConfig represents the application's configuration, loaded from a YAML file.
type SDKConfig struct {
	// ProxyURL is the URL of an optional proxy server to use for outbound requests.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// ForceModelPrefix requires explicit model prefixes (e.g., "teamA/gemini-3-pro-preview")
	// to target prefixed credentials. When false, unprefixed model requests may use prefixed
	// credentials as well.
	ForceModelPrefix bool `yaml:"force-model-prefix" json:"force-model-prefix"`

	// RequestLog enables or disables detailed request logging functionality.
	RequestLog bool `yaml:"request-log" json:"request-log"`

	// APIKeys is a list of client API keys accepted by this proxy server.
	// Each entry may optionally restrict which upstream auth_index values it can use.
	APIKeys []ClientAPIKeyConfig `yaml:"api-keys" json:"api-keys"`

	// PassthroughHeaders controls whether upstream response headers are forwarded to downstream clients.
	// Default is false (disabled).
	PassthroughHeaders bool `yaml:"passthrough-headers" json:"passthrough-headers"`

	// Streaming configures server-side streaming behavior (keep-alives and safe bootstrap retries).
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`

	// NonStreamKeepAliveInterval controls how often blank lines are emitted for non-streaming responses.
	// <= 0 disables keep-alives. Value is in seconds.
	NonStreamKeepAliveInterval int `yaml:"nonstream-keepalive-interval,omitempty" json:"nonstream-keepalive-interval,omitempty"`
}

// StreamingConfig holds server streaming behavior configuration.
type StreamingConfig struct {
	// KeepAliveSeconds controls how often the server emits SSE heartbeats (": keep-alive\n\n").
	// <= 0 disables keep-alives. Default is 0.
	KeepAliveSeconds int `yaml:"keepalive-seconds,omitempty" json:"keepalive-seconds,omitempty"`

	// BootstrapRetries controls how many times the server may retry a streaming request before any bytes are sent,
	// to allow auth rotation / transient recovery.
	// <= 0 disables bootstrap retries. Default is 0.
	BootstrapRetries int `yaml:"bootstrap-retries,omitempty" json:"bootstrap-retries,omitempty"`
}

// PlainAPIKeys returns the configured client key values without restriction metadata.
func (c *SDKConfig) PlainAPIKeys() []string {
	if c == nil || len(c.APIKeys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(c.APIKeys))
	out := make([]string, 0, len(c.APIKeys))
	for i := range c.APIKeys {
		key := strings.TrimSpace(c.APIKeys[i].Key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NormalizeAPIKeys trims, deduplicates, and canonicalizes configured client API keys.
func (c *SDKConfig) NormalizeAPIKeys() {
	if c == nil || len(c.APIKeys) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(c.APIKeys))
	out := make([]ClientAPIKeyConfig, 0, len(c.APIKeys))
	for i := range c.APIKeys {
		entry := c.APIKeys[i]
		entry.normalize()
		if entry.Key == "" {
			continue
		}
		if _, exists := seen[entry.Key]; exists {
			continue
		}
		seen[entry.Key] = struct{}{}
		out = append(out, entry)
	}
	c.APIKeys = out
}
