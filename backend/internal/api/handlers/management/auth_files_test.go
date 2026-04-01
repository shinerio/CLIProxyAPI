package management

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestManagementCallbackURL(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{
			name: "primary_https_uses_loopback_for_wildcard_host",
			cfg: &config.Config{
				Host: "0.0.0.0",
				Port: 8317,
				TLS:  config.TLSConfig{Enable: true},
			},
			want: "https://127.0.0.1:8317/callback",
		},
		{
			name: "primary_https_uses_bound_host",
			cfg: &config.Config{
				Host: "192.168.1.10",
				Port: 9443,
				TLS:  config.TLSConfig{Enable: true},
			},
			want: "https://192.168.1.10:9443/callback",
		},
		{
			name: "local_http_takes_precedence_when_enabled",
			cfg: &config.Config{
				Host: "0.0.0.0",
				Port: 9443,
				TLS:  config.TLSConfig{Enable: true},
				LocalHTTP: config.LocalHTTPConfig{
					Enable: true,
					Host:   "192.168.1.20",
					Port:   8318,
				},
			},
			want: "http://192.168.1.20:8318/callback",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{cfg: tc.cfg}
			got, err := h.managementCallbackURL("/callback")
			if err != nil {
				t.Fatalf("managementCallbackURL() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("managementCallbackURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
