package cliproxy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	internalusage "github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestServiceUsagePersistenceLifecycleFlushesAndRestoresSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialStats := internalusage.NewRequestStatistics()
	service := &Service{
		cfg: &config.Config{
			UsageStatisticsEnabled:              true,
			UsageStatisticsPersist:              true,
			UsageStatisticsPath:                 "state/usage-statistics.json",
			UsageStatisticsFlushIntervalSeconds: 3600,
		},
		configPath: configPath,
		usageStats: initialStats,
	}

	if err := service.startUsagePersistence(context.Background()); err != nil {
		t.Fatalf("start usage persistence: %v", err)
	}

	initialStats.Record(context.Background(), coreusage.Record{
		APIKey:      "test-key",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 3, 30, 15, 0, 0, 0, time.UTC),
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	})

	if err := service.stopUsagePersistence(context.Background()); err != nil {
		t.Fatalf("stop usage persistence: %v", err)
	}

	snapshotPath := filepath.Join(tmpDir, "state", "usage-statistics.json")
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if !strings.Contains(string(data), `"total_requests": 1`) {
		t.Fatalf("snapshot missing expected total_requests: %s", string(data))
	}

	restoreStats := internalusage.NewRequestStatistics()
	restoreService := &Service{
		cfg: &config.Config{
			UsageStatisticsEnabled:              true,
			UsageStatisticsPersist:              true,
			UsageStatisticsPath:                 "state/usage-statistics.json",
			UsageStatisticsFlushIntervalSeconds: 3600,
		},
		configPath: configPath,
		usageStats: restoreStats,
	}

	if err := restoreService.startUsagePersistence(context.Background()); err != nil {
		t.Fatalf("restore usage persistence: %v", err)
	}
	defer func() {
		if err := restoreService.stopUsagePersistence(context.Background()); err != nil {
			t.Fatalf("final stop usage persistence: %v", err)
		}
	}()

	snapshot := restoreStats.Snapshot()
	if snapshot.TotalRequests != 1 {
		t.Fatalf("restored total requests = %d, want 1", snapshot.TotalRequests)
	}
}
