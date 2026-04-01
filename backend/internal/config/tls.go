package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultTLSDirectoryName = "certificate"
	DefaultTLSCertFileName  = "fullchain.pem"
	DefaultTLSKeyFileName   = "privkey.pem"
	DefaultLocalHTTPHost    = "127.0.0.1"
	DefaultLocalHTTPPort    = 8318
)

// SanitizeTLS trims and normalizes TLS-related configuration.
func (cfg *Config) SanitizeTLS() {
	if cfg == nil {
		return
	}

	cfg.TLS.Cert = strings.TrimSpace(cfg.TLS.Cert)
	cfg.TLS.Key = strings.TrimSpace(cfg.TLS.Key)
}

// SanitizeLocalHTTP trims and normalizes secondary plain HTTP listener configuration.
func (cfg *Config) SanitizeLocalHTTP() {
	if cfg == nil {
		return
	}

	cfg.LocalHTTP.Host = strings.TrimSpace(cfg.LocalHTTP.Host)
	if cfg.LocalHTTP.Host == "" {
		cfg.LocalHTTP.Host = DefaultLocalHTTPHost
	}
	if cfg.LocalHTTP.Port <= 0 {
		cfg.LocalHTTP.Port = DefaultLocalHTTPPort
	}
}

// EffectiveCertPath returns the configured certificate path or the default export path.
func (cfg TLSConfig) EffectiveCertPath() string {
	if cert := strings.TrimSpace(cfg.Cert); cert != "" {
		return filepath.Clean(cert)
	}
	return filepath.Join(defaultTLSBaseDir(), DefaultTLSDirectoryName, DefaultTLSCertFileName)
}

// EffectiveKeyPath returns the configured private key path or the default export path.
func (cfg TLSConfig) EffectiveKeyPath() string {
	if key := strings.TrimSpace(cfg.Key); key != "" {
		return filepath.Clean(key)
	}
	return filepath.Join(defaultTLSBaseDir(), DefaultTLSDirectoryName, DefaultTLSKeyFileName)
}

// EffectiveHost returns the configured host or the default host.
func (cfg LocalHTTPConfig) EffectiveHost() string {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return DefaultLocalHTTPHost
	}
	return host
}

// EffectivePort returns the configured port or the default port.
func (cfg LocalHTTPConfig) EffectivePort() int {
	if cfg.Port > 0 {
		return cfg.Port
	}
	return DefaultLocalHTTPPort
}

// CallbackHost returns a browser-reachable host for callback redirects.
func CallbackHost(host string) string {
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	switch trimmed {
	case "", "0.0.0.0", "::", "localhost":
		return "127.0.0.1"
	default:
		return trimmed
	}
}

func defaultTLSBaseDir() string {
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return filepath.Dir(exe)
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		return filepath.Clean(wd)
	}
	return "."
}
