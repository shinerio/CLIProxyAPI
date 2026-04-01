package managementasset

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	log "github.com/sirupsen/logrus"
)

const (
	managementAssetName        = "management.html"
	localFrontendIndexFileName = "index.html"
)

// ManagementFileName exposes the control panel asset filename.
const ManagementFileName = managementAssetName

var currentConfigPtr atomic.Pointer[config.Config]

// SetCurrentConfig stores the latest configuration snapshot for management asset decisions.
func SetCurrentConfig(cfg *config.Config) {
	if cfg == nil {
		currentConfigPtr.Store(nil)
		return
	}
	currentConfigPtr.Store(cfg)
}

// StartAutoUpdater is kept as a compatibility no-op.
// Management assets are now served strictly from the local filesystem.
func StartAutoUpdater(_ context.Context, _ string) {
	log.Debug("management asset auto-updater disabled: using local files only")
}

// StaticDir resolves the directory that stores the management control panel asset.
func StaticDir(configFilePath string) string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_STATIC_PATH")); override != "" {
		cleaned := filepath.Clean(override)
		if strings.EqualFold(filepath.Base(cleaned), managementAssetName) {
			return filepath.Dir(cleaned)
		}
		return cleaned
	}

	if localPath := resolveLocalFrontendPath(configFilePath); localPath != "" {
		if pathLooksLikeFile(localPath) {
			return filepath.Dir(localPath)
		}
		return localPath
	}

	if writable := util.WritablePath(); writable != "" {
		return filepath.Join(writable, "static")
	}

	configFilePath = strings.TrimSpace(configFilePath)
	if configFilePath == "" {
		return ""
	}

	base := filepath.Dir(configFilePath)
	if fileInfo, err := os.Stat(configFilePath); err == nil && fileInfo.IsDir() {
		base = configFilePath
	}

	return filepath.Join(base, "static")
}

// FilePath resolves the absolute path to the local management control panel asset.
func FilePath(configFilePath string) string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_STATIC_PATH")); override != "" {
		cleaned := filepath.Clean(override)
		if strings.EqualFold(filepath.Base(cleaned), managementAssetName) {
			return cleaned
		}
		if entry := resolveFrontendEntry(cleaned); entry != "" {
			return entry
		}
		return filepath.Join(cleaned, ManagementFileName)
	}

	if localPath := resolveLocalFrontendPath(configFilePath); localPath != "" {
		if pathLooksLikeFile(localPath) {
			return localPath
		}
		if entry := resolveFrontendEntry(localPath); entry != "" {
			return entry
		}
		return filepath.Join(localPath, ManagementFileName)
	}

	root := StaticDir(configFilePath)
	if root == "" {
		return ""
	}
	if entry := resolveFrontendEntry(root); entry != "" {
		return entry
	}
	return filepath.Join(root, ManagementFileName)
}

// HasLocalFrontendOverride reports whether a config-driven local frontend path is active.
func HasLocalFrontendOverride(configFilePath string) bool {
	return strings.TrimSpace(resolveLocalFrontendPath(configFilePath)) != ""
}

// ResolveLocalAssetPath maps an incoming request path to a local frontend asset.
// Assets are resolved from the configured local frontend path when present, otherwise
// from the install-local static directory.
func ResolveLocalAssetPath(configFilePath string, requestPath string) string {
	root := resolveFrontendRoot(configFilePath)
	if root == "" {
		return ""
	}

	cleanedPath := path.Clean("/" + strings.TrimSpace(requestPath))
	if cleanedPath == "." || cleanedPath == "/" {
		if pathLooksLikeFile(root) {
			return root
		}
		return resolveFrontendEntry(root)
	}

	if pathLooksLikeFile(root) {
		if cleanedPath == "/"+filepath.Base(root) || cleanedPath == "/"+managementAssetName || cleanedPath == "/"+localFrontendIndexFileName {
			return root
		}
		return ""
	}

	rel := strings.TrimPrefix(cleanedPath, "/")
	if rel == "" {
		return resolveFrontendEntry(root)
	}

	if rel == managementAssetName || rel == localFrontendIndexFileName {
		return resolveFrontendEntry(root)
	}

	candidate := filepath.Join(root, filepath.FromSlash(rel))
	relativeToRoot, err := filepath.Rel(root, candidate)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return ""
	}

	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return ""
	}

	return candidate
}

// EnsureLatestManagementHTML is kept for compatibility and now only checks whether
// a local management asset exists on disk.
func EnsureLatestManagementHTML(_ context.Context, staticDir string, _ string, _ string) bool {
	staticDir = strings.TrimSpace(staticDir)
	if staticDir == "" {
		return false
	}
	return resolveFrontendEntry(staticDir) != ""
}

func resolveFrontendRoot(configFilePath string) string {
	if localPath := resolveLocalFrontendPath(configFilePath); localPath != "" {
		return localPath
	}
	return StaticDir(configFilePath)
}

func resolveFrontendEntry(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	if pathLooksLikeFile(root) {
		if info, err := os.Stat(root); err == nil && !info.IsDir() {
			return root
		}
		return ""
	}
	for _, name := range []string{managementAssetName, localFrontendIndexFileName} {
		candidate := filepath.Join(root, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func resolveLocalFrontendPath(configFilePath string) string {
	cfg := currentConfigPtr.Load()
	if cfg == nil {
		return ""
	}

	localPath := strings.TrimSpace(cfg.RemoteManagement.LocalFrontendPath)
	if localPath == "" {
		return ""
	}

	if filepath.IsAbs(localPath) {
		return filepath.Clean(localPath)
	}

	base := strings.TrimSpace(configFilePath)
	if base == "" {
		if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
			return filepath.Join(filepath.Clean(wd), filepath.Clean(localPath))
		}
		return filepath.Clean(localPath)
	}

	if info, err := os.Stat(base); err == nil && info.IsDir() {
		return filepath.Join(base, filepath.Clean(localPath))
	}

	return filepath.Join(filepath.Dir(base), filepath.Clean(localPath))
}

func pathLooksLikeFile(candidate string) bool {
	if strings.EqualFold(filepath.Base(candidate), managementAssetName) || strings.EqualFold(filepath.Base(candidate), localFrontendIndexFileName) {
		return true
	}

	if info, err := os.Stat(candidate); err == nil {
		return !info.IsDir()
	}

	return filepath.Ext(candidate) != ""
}
