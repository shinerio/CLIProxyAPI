package registry

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
)

//go:embed models/models.json
var embeddedModelsJSON []byte

type modelStore struct {
	mu   sync.RWMutex
	data *staticModelsJSON
}

var modelsCatalogStore = &modelStore{}

var updaterOnce sync.Once

// ModelRefreshCallback is kept for compatibility.
// The catalog is now loaded from embedded local data only.
type ModelRefreshCallback func(changedProviders []string)

var refreshCallbackMu sync.Mutex

// SetModelRefreshCallback keeps the previous API surface for callers that register hooks.
func SetModelRefreshCallback(cb ModelRefreshCallback) {
	refreshCallbackMu.Lock()
	defer refreshCallbackMu.Unlock()
	if cb != nil {
		log.Debug("model refresh callback registered; remote model refresh is disabled")
	}
}

func init() {
	if err := loadModelsFromBytes(embeddedModelsJSON, "embed"); err != nil {
		panic(fmt.Sprintf("registry: failed to parse embedded models.json: %v", err))
	}
}

// StartModelsUpdater is kept as a compatibility no-op.
// Model definitions are now served only from the embedded catalog.
func StartModelsUpdater(_ context.Context) {
	updaterOnce.Do(func() {
		log.Debug("model updater disabled: using embedded catalog only")
	})
}

func loadModelsFromBytes(data []byte, source string) error {
	var parsed staticModelsJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse %s models catalog: %w", source, err)
	}

	modelsCatalogStore.mu.Lock()
	modelsCatalogStore.data = &parsed
	modelsCatalogStore.mu.Unlock()
	return nil
}

func getModels() *staticModelsJSON {
	modelsCatalogStore.mu.RLock()
	defer modelsCatalogStore.mu.RUnlock()
	if modelsCatalogStore.data == nil {
		return &staticModelsJSON{}
	}
	return modelsCatalogStore.data
}
