package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const currentSnapshotVersion = 1

type persistedSnapshot struct {
	Version int                `json:"version"`
	SavedAt time.Time          `json:"saved_at"`
	Usage   StatisticsSnapshot `json:"usage"`
}

// SnapshotPersistence periodically flushes usage statistics to disk and restores
// them on startup.
type SnapshotPersistence struct {
	stats         *RequestStatistics
	path          string
	flushInterval time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

// NewSnapshotPersistence creates a persistence manager for usage snapshots.
func NewSnapshotPersistence(stats *RequestStatistics, path string, flushInterval time.Duration) *SnapshotPersistence {
	return &SnapshotPersistence{
		stats:         stats,
		path:          filepath.Clean(path),
		flushInterval: flushInterval,
	}
}

// Load restores a previously persisted snapshot into the in-memory statistics store.
func (p *SnapshotPersistence) Load(_ context.Context) error {
	if p == nil || p.stats == nil || p.path == "" {
		return nil
	}

	data, err := os.ReadFile(p.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("usage persistence: read snapshot: %w", err)
	}

	var payload persistedSnapshot
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("usage persistence: decode snapshot: %w", err)
	}
	if payload.Version != 0 && payload.Version != currentSnapshotVersion {
		return fmt.Errorf("usage persistence: unsupported snapshot version %d", payload.Version)
	}

	p.stats.MergeSnapshot(payload.Usage)
	return nil
}

// Start loads any existing snapshot and launches the periodic flush loop.
func (p *SnapshotPersistence) Start(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if err := p.Load(ctx); err != nil {
		return err
	}
	if p.flushInterval <= 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	p.cancel = cancel
	p.done = done
	p.running = true

	go p.run(workerCtx, done)
	return nil
}

func (p *SnapshotPersistence) run(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = p.Flush(context.Background())
		}
	}
}

// Stop terminates the periodic flush loop and writes the latest snapshot once.
func (p *SnapshotPersistence) Stop(ctx context.Context) error {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	wasRunning := p.running
	p.cancel = nil
	p.done = nil
	p.running = false
	p.mu.Unlock()

	if wasRunning && cancel != nil {
		cancel()
	}
	if done != nil {
		if ctx == nil {
			<-done
		} else {
			select {
			case <-done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return p.Flush(ctx)
}

// Flush writes the current in-memory snapshot to disk using atomic replace.
func (p *SnapshotPersistence) Flush(_ context.Context) error {
	if p == nil || p.stats == nil || p.path == "" {
		return nil
	}

	payload := persistedSnapshot{
		Version: currentSnapshotVersion,
		SavedAt: time.Now().UTC(),
		Usage:   p.stats.Snapshot(),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("usage persistence: encode snapshot: %w", err)
	}

	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("usage persistence: create dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "usage-statistics-*.tmp")
	if err != nil {
		return fmt.Errorf("usage persistence: create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("usage persistence: write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("usage persistence: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, p.path); err != nil {
		return fmt.Errorf("usage persistence: rename temp file: %w", err)
	}
	cleanup = false

	return nil
}
