package managementasset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestFilePathAndStaticDirPreferConfiguredLocalFrontendDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", "config.yaml")
	localDir := filepath.Join(filepath.Dir(configPath), "frontend")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	localFile := filepath.Join(localDir, "management.html")
	if err := os.WriteFile(localFile, []byte("management"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := &config.Config{
		RemoteManagement: config.RemoteManagement{
			LocalFrontendPath: "frontend",
		},
	}

	SetCurrentConfig(cfg)
	defer SetCurrentConfig(nil)

	wantDir := filepath.Join(filepath.Dir(configPath), "frontend")
	if got := StaticDir(configPath); got != wantDir {
		t.Fatalf("StaticDir() = %q, want %q", got, wantDir)
	}

	wantFile := localFile
	if got := FilePath(configPath); got != wantFile {
		t.Fatalf("FilePath() = %q, want %q", got, wantFile)
	}
}

func TestFilePathAndResolveLocalAssetPathSupportConfiguredLocalFrontendFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	localFile := filepath.Join(tmpDir, "ui", "management.html")
	cfg := &config.Config{
		RemoteManagement: config.RemoteManagement{
			LocalFrontendPath: localFile,
		},
	}

	SetCurrentConfig(cfg)
	defer SetCurrentConfig(nil)

	if got := StaticDir(configPath); got != filepath.Dir(localFile) {
		t.Fatalf("StaticDir() = %q, want %q", got, filepath.Dir(localFile))
	}

	if got := FilePath(configPath); got != localFile {
		t.Fatalf("FilePath() = %q, want %q", got, localFile)
	}

	if got := ResolveLocalAssetPath(configPath, "/management.html"); got != localFile {
		t.Fatalf("ResolveLocalAssetPath() = %q, want %q", got, localFile)
	}
}

func TestResolveLocalAssetPathServesFilesWithinConfiguredDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "frontend")
	assetsDir := filepath.Join(localDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	indexPath := filepath.Join(localDir, "management.html")
	if err := os.WriteFile(indexPath, []byte("index"), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	assetPath := filepath.Join(assetsDir, "main.js")
	if err := os.WriteFile(assetPath, []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatalf("WriteFile(asset) error = %v", err)
	}

	cfg := &config.Config{
		RemoteManagement: config.RemoteManagement{
			LocalFrontendPath: localDir,
		},
	}

	SetCurrentConfig(cfg)
	defer SetCurrentConfig(nil)

	if !HasLocalFrontendOverride(filepath.Join(tmpDir, "config.yaml")) {
		t.Fatalf("HasLocalFrontendOverride() = false, want true")
	}

	if got := ResolveLocalAssetPath(filepath.Join(tmpDir, "config.yaml"), "/management.html"); got != indexPath {
		t.Fatalf("ResolveLocalAssetPath(index) = %q, want %q", got, indexPath)
	}

	if got := ResolveLocalAssetPath(filepath.Join(tmpDir, "config.yaml"), "/assets/main.js"); got != assetPath {
		t.Fatalf("ResolveLocalAssetPath(asset) = %q, want %q", got, assetPath)
	}

	if got := ResolveLocalAssetPath(filepath.Join(tmpDir, "config.yaml"), "/../secret.txt"); got != "" {
		t.Fatalf("ResolveLocalAssetPath(traversal) = %q, want empty", got)
	}
}

func TestLocalFrontendDirectoryFallsBackToIndexHTML(t *testing.T) {
	tmpDir := t.TempDir()
	localDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	indexPath := filepath.Join(localDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("index"), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}

	cfg := &config.Config{
		RemoteManagement: config.RemoteManagement{
			LocalFrontendPath: localDir,
		},
	}

	configPath := filepath.Join(tmpDir, "config.yaml")
	SetCurrentConfig(cfg)
	defer SetCurrentConfig(nil)

	if got := FilePath(configPath); got != indexPath {
		t.Fatalf("FilePath() = %q, want %q", got, indexPath)
	}

	if got := ResolveLocalAssetPath(configPath, "/management.html"); got != indexPath {
		t.Fatalf("ResolveLocalAssetPath(/management.html) = %q, want %q", got, indexPath)
	}

	if got := ResolveLocalAssetPath(configPath, "/"); got != indexPath {
		t.Fatalf("ResolveLocalAssetPath(/) = %q, want %q", got, indexPath)
	}

	if got := ResolveLocalAssetPath(configPath, "/index.html"); got != indexPath {
		t.Fatalf("ResolveLocalAssetPath(/index.html) = %q, want %q", got, indexPath)
	}
}

func TestResolveLocalAssetPathFallsBackToStaticDirWithoutOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	staticDir := filepath.Join(tmpDir, "static")
	assetsDir := filepath.Join(staticDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	indexPath := filepath.Join(staticDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("index"), 0o644); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	assetPath := filepath.Join(assetsDir, "app.js")
	if err := os.WriteFile(assetPath, []byte("console.log('local static')"), 0o644); err != nil {
		t.Fatalf("WriteFile(asset) error = %v", err)
	}

	SetCurrentConfig(&config.Config{})
	defer SetCurrentConfig(nil)

	if got := FilePath(configPath); got != indexPath {
		t.Fatalf("FilePath() = %q, want %q", got, indexPath)
	}

	if got := ResolveLocalAssetPath(configPath, "/"); got != indexPath {
		t.Fatalf("ResolveLocalAssetPath(/) = %q, want %q", got, indexPath)
	}

	if got := ResolveLocalAssetPath(configPath, "/assets/app.js"); got != assetPath {
		t.Fatalf("ResolveLocalAssetPath(/assets/app.js) = %q, want %q", got, assetPath)
	}
}
