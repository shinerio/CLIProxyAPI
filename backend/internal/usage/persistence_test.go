package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestLoadUsageSnapshotFromDisk(t *testing.T) {
	stats := NewRequestStatistics()
	path := filepath.Join(t.TempDir(), "usage-statistics.json")

	payload := persistedSnapshot{
		Version: 1,
		Usage: StatisticsSnapshot{
			TotalRequests: 1,
			SuccessCount:  1,
			TotalTokens:   30,
			APIs: map[string]APISnapshot{
				"test-key": {
					TotalRequests: 1,
					TotalTokens:   30,
					Models: map[string]ModelSnapshot{
						"gpt-5.4": {
							TotalRequests: 1,
							TotalTokens:   30,
							Details: []RequestDetail{{
								Timestamp: time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC),
								Source:    "unit-test",
								AuthIndex: "0",
								Tokens: TokenStats{
									InputTokens:  10,
									OutputTokens: 20,
									TotalTokens:  30,
								},
							}},
						},
					},
				},
			},
			RequestsByDay: map[string]int64{"2026-03-30": 1},
			RequestsByHour: map[string]int64{
				"12": 1,
			},
			TokensByDay: map[string]int64{"2026-03-30": 30},
			TokensByHour: map[string]int64{
				"12": 30,
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	persistence := NewSnapshotPersistence(stats, path, time.Hour)
	if err := persistence.Load(context.Background()); err != nil {
		t.Fatalf("load snapshot: %v", err)
	}

	snapshot := stats.Snapshot()
	if snapshot.TotalRequests != 1 {
		t.Fatalf("total requests = %d, want 1", snapshot.TotalRequests)
	}
	details := snapshot.APIs["test-key"].Models["gpt-5.4"].Details
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}
}

func TestUsagePersistenceStopFlushesLatestSnapshot(t *testing.T) {
	stats := NewRequestStatistics()
	path := filepath.Join(t.TempDir(), "usage-statistics.json")
	persistence := NewSnapshotPersistence(stats, path, time.Hour)

	if err := persistence.Start(context.Background()); err != nil {
		t.Fatalf("start persistence: %v", err)
	}

	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "test-key",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 3, 30, 13, 0, 0, 0, time.UTC),
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	})

	if err := persistence.Stop(context.Background()); err != nil {
		t.Fatalf("stop persistence: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	var saved persistedSnapshot
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if saved.Usage.TotalRequests != 1 {
		t.Fatalf("total requests = %d, want 1", saved.Usage.TotalRequests)
	}
}

func TestUsagePersistencePeriodicFlushWritesSnapshot(t *testing.T) {
	stats := NewRequestStatistics()
	path := filepath.Join(t.TempDir(), "usage-statistics.json")
	persistence := NewSnapshotPersistence(stats, path, 10*time.Millisecond)

	if err := persistence.Start(context.Background()); err != nil {
		t.Fatalf("start persistence: %v", err)
	}
	defer func() {
		if err := persistence.Stop(context.Background()); err != nil {
			t.Fatalf("stop persistence: %v", err)
		}
	}()

	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "test-key",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 3, 30, 14, 0, 0, 0, time.UTC),
		Detail: coreusage.Detail{
			InputTokens:  1,
			OutputTokens: 2,
			TotalTokens:  3,
		},
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			var saved persistedSnapshot
			if err := json.Unmarshal(data, &saved); err == nil && saved.Usage.TotalRequests == 1 {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("snapshot file %s was not written by periodic flush", path)
}
