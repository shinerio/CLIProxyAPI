package usage

import (
	"context"
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestRequestStatisticsRecordIncludesLatency(t *testing.T) {
	stats := NewRequestStatistics()
	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "test-key",
		Model:       "gpt-5.4",
		RequestedAt: time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
		Latency:     1500 * time.Millisecond,
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	})

	snapshot := stats.Snapshot()
	details := snapshot.APIs["test-key"].Models["gpt-5.4"].Details
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}
	if details[0].LatencyMs != 1500 {
		t.Fatalf("latency_ms = %d, want 1500", details[0].LatencyMs)
	}
}

func TestRequestStatisticsMergeSnapshotDedupIgnoresLatency(t *testing.T) {
	stats := NewRequestStatistics()
	timestamp := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	first := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"test-key": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{{
							Timestamp: timestamp,
							LatencyMs: 0,
							Source:    "user@example.com",
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
	}
	second := StatisticsSnapshot{
		APIs: map[string]APISnapshot{
			"test-key": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						Details: []RequestDetail{{
							Timestamp: timestamp,
							LatencyMs: 2500,
							Source:    "user@example.com",
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
	}

	result := stats.MergeSnapshot(first)
	if result.Added != 1 || result.Skipped != 0 {
		t.Fatalf("first merge = %+v, want added=1 skipped=0", result)
	}

	result = stats.MergeSnapshot(second)
	if result.Added != 0 || result.Skipped != 1 {
		t.Fatalf("second merge = %+v, want added=0 skipped=1", result)
	}

	snapshot := stats.Snapshot()
	details := snapshot.APIs["test-key"].Models["gpt-5.4"].Details
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}
}

func TestRequestStatisticsRetainsOnlyLatest500DetailsButKeepsTotals(t *testing.T) {
	stats := NewRequestStatistics()
	baseTime := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 600; i++ {
		stats.Record(context.Background(), coreusage.Record{
			APIKey:      "test-key",
			Model:       "gpt-5.4",
			RequestedAt: baseTime.Add(time.Duration(i) * time.Minute),
			Detail: coreusage.Detail{
				InputTokens:  1,
				OutputTokens: 2,
				TotalTokens:  3,
			},
		})
	}

	snapshot := stats.Snapshot()
	if snapshot.TotalRequests != 600 {
		t.Fatalf("total requests = %d, want 600", snapshot.TotalRequests)
	}
	if snapshot.TotalTokens != 1800 {
		t.Fatalf("total tokens = %d, want 1800", snapshot.TotalTokens)
	}
	details := snapshot.APIs["test-key"].Models["gpt-5.4"].Details
	if len(details) != 500 {
		t.Fatalf("details len = %d, want 500", len(details))
	}
	if got := details[0].Timestamp; !got.Equal(baseTime.Add(100 * time.Minute)) {
		t.Fatalf("oldest retained timestamp = %v, want %v", got, baseTime.Add(100*time.Minute))
	}
}

func TestMergeSnapshotWithAggregatesPreservesTotalsBeyondRetainedDetails(t *testing.T) {
	stats := NewRequestStatistics()
	timestamp := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)

	result := stats.MergeSnapshot(StatisticsSnapshot{
		TotalRequests: 1000,
		SuccessCount:  990,
		FailureCount:  10,
		TotalTokens:   3000,
		TokenStats: TokenStats{
			InputTokens:  1000,
			OutputTokens: 2000,
			TotalTokens:  3000,
		},
		RequestsByDay: map[string]int64{"2026-04-02": 1000},
		TokensByDay:   map[string]int64{"2026-04-02": 3000},
		APIs: map[string]APISnapshot{
			"test-key": {
				Models: map[string]ModelSnapshot{
					"gpt-5.4": {
						TotalRequests: 1000,
						SuccessCount:  990,
						FailureCount:  10,
						TotalTokens:   3000,
						TokenStats: TokenStats{
							InputTokens:  1000,
							OutputTokens: 2000,
							TotalTokens:  3000,
						},
						Details: []RequestDetail{{
							Timestamp: timestamp,
							Source:    "user@example.com",
							AuthIndex: "0",
							Tokens: TokenStats{
								InputTokens:  1,
								OutputTokens: 2,
								TotalTokens:  3,
							},
						}},
					},
				},
			},
		},
	})
	if result.Added != 1 || result.Skipped != 0 {
		t.Fatalf("merge result = %+v, want added=1 skipped=0", result)
	}

	snapshot := stats.Snapshot()
	if snapshot.TotalRequests != 1000 {
		t.Fatalf("total requests = %d, want 1000", snapshot.TotalRequests)
	}
	if snapshot.TotalTokens != 3000 {
		t.Fatalf("total tokens = %d, want 3000", snapshot.TotalTokens)
	}
	model := snapshot.APIs["test-key"].Models["gpt-5.4"]
	if model.TotalRequests != 1000 {
		t.Fatalf("model total requests = %d, want 1000", model.TotalRequests)
	}
	if len(model.Details) != 1 {
		t.Fatalf("details len = %d, want 1", len(model.Details))
	}
}
