package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTLSConfigEffectivePathsUseCertificateDirectory(t *testing.T) {
	cfg := TLSConfig{}

	certPath := filepath.ToSlash(cfg.EffectiveCertPath())
	keyPath := filepath.ToSlash(cfg.EffectiveKeyPath())

	if !strings.HasSuffix(certPath, "/"+DefaultTLSDirectoryName+"/"+DefaultTLSCertFileName) {
		t.Fatalf("EffectiveCertPath() = %q", certPath)
	}
	if !strings.HasSuffix(keyPath, "/"+DefaultTLSDirectoryName+"/"+DefaultTLSKeyFileName) {
		t.Fatalf("EffectiveKeyPath() = %q", keyPath)
	}
}

func TestConfigSanitizeTLSTrimsCertAndKey(t *testing.T) {
	cfg := &Config{
		TLS: TLSConfig{
			Cert: "  cert.pem  ",
			Key:  "  key.pem  ",
		},
		LocalHTTP: LocalHTTPConfig{
			Host: " 192.168.1.10 ",
			Port: 0,
		},
	}

	cfg.SanitizeTLS()
	cfg.SanitizeLocalHTTP()

	if cfg.TLS.Cert != "cert.pem" {
		t.Fatalf("TLS.Cert = %q", cfg.TLS.Cert)
	}
	if cfg.TLS.Key != "key.pem" {
		t.Fatalf("TLS.Key = %q", cfg.TLS.Key)
	}
	if cfg.LocalHTTP.Host != "192.168.1.10" {
		t.Fatalf("LocalHTTP.Host = %q", cfg.LocalHTTP.Host)
	}
	if cfg.LocalHTTP.Port != DefaultLocalHTTPPort {
		t.Fatalf("LocalHTTP.Port = %d", cfg.LocalHTTP.Port)
	}
}

func TestCallbackHostUsesLoopbackForWildcardHosts(t *testing.T) {
	testCases := map[string]string{
		"":            "127.0.0.1",
		"0.0.0.0":     "127.0.0.1",
		"::":          "127.0.0.1",
		"[::]":        "127.0.0.1",
		"localhost":   "127.0.0.1",
		"192.168.1.5": "192.168.1.5",
	}

	for input, want := range testCases {
		if got := CallbackHost(input); got != want {
			t.Fatalf("CallbackHost(%q) = %q, want %q", input, got, want)
		}
	}
}
