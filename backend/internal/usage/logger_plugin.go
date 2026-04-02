// Package usage provides usage tracking and logging functionality for the CLI Proxy API server.
// It includes plugins for monitoring API usage, token consumption, and other metrics
// to help with observability and billing purposes.
package usage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

const (
	defaultRetainedRequestDetails = 500
	recentMinuteBucketLimit       = 7 * 24 * 60
	recentHourBucketLimit         = 7 * 24
	serviceHealthBucketLimit      = 7 * 96
)

var statisticsEnabled atomic.Bool
var retainedRequestDetailsLimit atomic.Int64

func init() {
	statisticsEnabled.Store(true)
	retainedRequestDetailsLimit.Store(defaultRetainedRequestDetails)
	coreusage.RegisterPlugin(NewLoggerPlugin())
}

// LoggerPlugin collects in-memory request statistics for usage analysis.
// It implements coreusage.Plugin to receive usage records emitted by the runtime.
type LoggerPlugin struct {
	stats *RequestStatistics
}

// NewLoggerPlugin constructs a new logger plugin instance.
func NewLoggerPlugin() *LoggerPlugin { return &LoggerPlugin{stats: defaultRequestStatistics} }

// HandleUsage implements coreusage.Plugin.
func (p *LoggerPlugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if !statisticsEnabled.Load() {
		return
	}
	if p == nil || p.stats == nil {
		return
	}
	p.stats.Record(ctx, record)
}

// SetStatisticsEnabled toggles whether in-memory statistics are recorded.
func SetStatisticsEnabled(enabled bool) { statisticsEnabled.Store(enabled) }

// StatisticsEnabled reports the current recording state.
func StatisticsEnabled() bool { return statisticsEnabled.Load() }

// SetRetainedRequestDetailsLimit configures how many recent request details remain in memory.
// Values below zero reset to the default.
func SetRetainedRequestDetailsLimit(limit int) {
	if limit < 0 {
		limit = defaultRetainedRequestDetails
	}
	retainedRequestDetailsLimit.Store(int64(limit))
}

// RetainedRequestDetailsLimit reports the current in-memory detail retention limit.
func RetainedRequestDetailsLimit() int { return int(retainedRequestDetailsLimit.Load()) }

type detailRef struct {
	apiName   string
	modelName string
	sequence  uint64
}

// RequestStatistics maintains aggregated request metrics in memory.
type RequestStatistics struct {
	mu sync.RWMutex

	totalRequests        int64
	successCount         int64
	failureCount         int64
	totalTokens          int64
	tokenStats           TokenStats
	maxRequestsPerMinute int64
	maxTokensPerMinute   int64

	apis map[string]*apiStats

	requestsByDay  map[string]int64
	requestsByHour map[int]int64
	tokensByDay    map[string]int64
	tokensByHour   map[int]int64

	recentRequestsByMinute map[string]int64
	recentTokensByMinute   map[string]int64
	serviceHealthByBucket  map[string]ServiceHealthSnapshot

	detailOrder  []detailRef
	nextSequence uint64
}

// apiStats holds aggregated metrics for a single API key.
type apiStats struct {
	TotalRequests int64
	SuccessCount  int64
	FailureCount  int64
	TotalTokens   int64
	TokenStats    TokenStats
	Models        map[string]*modelStats
}

// modelStats holds aggregated metrics for a specific model within an API.
type modelStats struct {
	TotalRequests    int64
	SuccessCount     int64
	FailureCount     int64
	TotalTokens      int64
	TokenStats       TokenStats
	RequestsByDay    map[string]int64
	RequestsByHour   map[string]int64
	TokensByDay      map[string]int64
	TokensByHour     map[string]int64
	TokenStatsByDay  map[string]TokenStats
	TokenStatsByHour map[string]TokenStats
	Details          []RequestDetail
}

// RequestDetail stores the timestamp, latency, and token usage for a single request.
type RequestDetail struct {
	Timestamp time.Time  `json:"timestamp"`
	LatencyMs int64      `json:"latency_ms"`
	Source    string     `json:"source"`
	AuthIndex string     `json:"auth_index"`
	Tokens    TokenStats `json:"tokens"`
	Failed    bool       `json:"failed"`
	Sequence  uint64     `json:"sequence,omitempty"`
}

// TokenStats captures the token usage breakdown for a request.
type TokenStats struct {
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	CachedTokens    int64 `json:"cached_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
}

// ServiceHealthSnapshot summarises success and failure totals for a quarter-hour bucket.
type ServiceHealthSnapshot struct {
	SuccessCount int64 `json:"success_count"`
	FailureCount int64 `json:"failure_count"`
}

// StatisticsSnapshot represents an immutable view of the aggregated metrics.
type StatisticsSnapshot struct {
	TotalRequests        int64      `json:"total_requests"`
	SuccessCount         int64      `json:"success_count"`
	FailureCount         int64      `json:"failure_count"`
	TotalTokens          int64      `json:"total_tokens"`
	TokenStats           TokenStats `json:"token_stats"`
	MaxRequestsPerMinute int64      `json:"max_requests_per_minute,omitempty"`
	MaxTokensPerMinute   int64      `json:"max_tokens_per_minute,omitempty"`

	APIs map[string]APISnapshot `json:"apis"`

	RequestsByDay  map[string]int64 `json:"requests_by_day"`
	RequestsByHour map[string]int64 `json:"requests_by_hour"`
	TokensByDay    map[string]int64 `json:"tokens_by_day"`
	TokensByHour   map[string]int64 `json:"tokens_by_hour"`

	RecentRequestsByMinute map[string]int64                 `json:"recent_requests_by_minute,omitempty"`
	RecentTokensByMinute   map[string]int64                 `json:"recent_tokens_by_minute,omitempty"`
	ServiceHealthByBucket  map[string]ServiceHealthSnapshot `json:"service_health_by_quarter_hour,omitempty"`
}

// APISnapshot summarises metrics for a single API key.
type APISnapshot struct {
	TotalRequests int64                    `json:"total_requests"`
	SuccessCount  int64                    `json:"success_count"`
	FailureCount  int64                    `json:"failure_count"`
	TotalTokens   int64                    `json:"total_tokens"`
	TokenStats    TokenStats               `json:"token_stats"`
	Models        map[string]ModelSnapshot `json:"models"`
}

// ModelSnapshot summarises metrics for a specific model.
type ModelSnapshot struct {
	TotalRequests    int64                 `json:"total_requests"`
	SuccessCount     int64                 `json:"success_count"`
	FailureCount     int64                 `json:"failure_count"`
	TotalTokens      int64                 `json:"total_tokens"`
	TokenStats       TokenStats            `json:"token_stats"`
	RequestsByDay    map[string]int64      `json:"requests_by_day,omitempty"`
	RequestsByHour   map[string]int64      `json:"requests_by_hour,omitempty"`
	TokensByDay      map[string]int64      `json:"tokens_by_day,omitempty"`
	TokensByHour     map[string]int64      `json:"tokens_by_hour,omitempty"`
	TokenStatsByDay  map[string]TokenStats `json:"token_stats_by_day,omitempty"`
	TokenStatsByHour map[string]TokenStats `json:"token_stats_by_hour,omitempty"`
	Details          []RequestDetail       `json:"details"`
}

var defaultRequestStatistics = NewRequestStatistics()

// GetRequestStatistics returns the shared statistics store.
func GetRequestStatistics() *RequestStatistics { return defaultRequestStatistics }

// NewRequestStatistics constructs an empty statistics store.
func NewRequestStatistics() *RequestStatistics {
	return &RequestStatistics{
		apis:                   make(map[string]*apiStats),
		requestsByDay:          make(map[string]int64),
		requestsByHour:         make(map[int]int64),
		tokensByDay:            make(map[string]int64),
		tokensByHour:           make(map[int]int64),
		recentRequestsByMinute: make(map[string]int64),
		recentTokensByMinute:   make(map[string]int64),
		serviceHealthByBucket:  make(map[string]ServiceHealthSnapshot),
	}
}

// Record ingests a new usage record and updates the aggregates.
func (s *RequestStatistics) Record(ctx context.Context, record coreusage.Record) {
	if s == nil {
		return
	}
	if !statisticsEnabled.Load() {
		return
	}

	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	timestamp = timestamp.UTC()

	detailTokens := normaliseDetail(record.Detail)
	totalTokens := detailTokens.TotalTokens
	statsKey := record.APIKey
	if statsKey == "" {
		statsKey = resolveAPIIdentifier(ctx, record)
	}
	failed := record.Failed
	if !failed {
		failed = !resolveSuccess(ctx)
	}
	success := !failed
	modelName := strings.TrimSpace(record.Model)
	if modelName == "" {
		modelName = "unknown"
	}

	requestDetail := RequestDetail{
		Timestamp: timestamp,
		LatencyMs: normaliseLatency(record.Latency),
		Source:    record.Source,
		AuthIndex: record.AuthIndex,
		Tokens:    detailTokens,
		Failed:    failed,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalRequests++
	if success {
		s.successCount++
	} else {
		s.failureCount++
	}
	s.totalTokens += totalTokens
	s.tokenStats = addTokenStats(s.tokenStats, detailTokens)

	apiEntry := s.ensureAPIStatsLocked(statsKey)
	modelEntry := s.ensureModelStatsLocked(apiEntry, modelName)

	s.applyAggregateToAPIAndModelLocked(apiEntry, modelEntry, detailTokens, success)
	s.appendDetailLocked(statsKey, modelName, modelEntry, requestDetail)
	s.updateTopLevelBucketsLocked(timestamp, detailTokens, success)
	s.updateModelBucketsLocked(modelEntry, timestamp, detailTokens)
}

// Snapshot returns a copy of the aggregated metrics for external consumption.
func (s *RequestStatistics) Snapshot() StatisticsSnapshot {
	result := StatisticsSnapshot{}
	if s == nil {
		return result
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result.TotalRequests = s.totalRequests
	result.SuccessCount = s.successCount
	result.FailureCount = s.failureCount
	result.TotalTokens = s.totalTokens
	result.TokenStats = s.tokenStats
	result.MaxRequestsPerMinute = s.maxRequestsPerMinute
	result.MaxTokensPerMinute = s.maxTokensPerMinute

	result.APIs = make(map[string]APISnapshot, len(s.apis))
	for apiName, stats := range s.apis {
		apiSnapshot := APISnapshot{
			TotalRequests: stats.TotalRequests,
			SuccessCount:  stats.SuccessCount,
			FailureCount:  stats.FailureCount,
			TotalTokens:   stats.TotalTokens,
			TokenStats:    stats.TokenStats,
			Models:        make(map[string]ModelSnapshot, len(stats.Models)),
		}
		for modelName, modelStatsValue := range stats.Models {
			requestDetails := make([]RequestDetail, len(modelStatsValue.Details))
			copy(requestDetails, modelStatsValue.Details)
			apiSnapshot.Models[modelName] = ModelSnapshot{
				TotalRequests:    modelStatsValue.TotalRequests,
				SuccessCount:     modelStatsValue.SuccessCount,
				FailureCount:     modelStatsValue.FailureCount,
				TotalTokens:      modelStatsValue.TotalTokens,
				TokenStats:       modelStatsValue.TokenStats,
				RequestsByDay:    cloneInt64Map(modelStatsValue.RequestsByDay),
				RequestsByHour:   cloneInt64Map(modelStatsValue.RequestsByHour),
				TokensByDay:      cloneInt64Map(modelStatsValue.TokensByDay),
				TokensByHour:     cloneInt64Map(modelStatsValue.TokensByHour),
				TokenStatsByDay:  cloneTokenStatsMap(modelStatsValue.TokenStatsByDay),
				TokenStatsByHour: cloneTokenStatsMap(modelStatsValue.TokenStatsByHour),
				Details:          requestDetails,
			}
		}
		result.APIs[apiName] = apiSnapshot
	}

	result.RequestsByDay = cloneInt64Map(s.requestsByDay)

	result.RequestsByHour = make(map[string]int64, len(s.requestsByHour))
	for hour, v := range s.requestsByHour {
		result.RequestsByHour[formatLegacyHour(hour)] = v
	}

	result.TokensByDay = cloneInt64Map(s.tokensByDay)

	result.TokensByHour = make(map[string]int64, len(s.tokensByHour))
	for hour, v := range s.tokensByHour {
		result.TokensByHour[formatLegacyHour(hour)] = v
	}

	result.RecentRequestsByMinute = cloneInt64Map(s.recentRequestsByMinute)
	result.RecentTokensByMinute = cloneInt64Map(s.recentTokensByMinute)
	result.ServiceHealthByBucket = cloneServiceHealthMap(s.serviceHealthByBucket)

	return result
}

type MergeResult struct {
	Added   int64 `json:"added"`
	Skipped int64 `json:"skipped"`
}

// MergeSnapshot merges an exported statistics snapshot into the current store.
// Existing data is preserved and duplicate request details are skipped.
func (s *RequestStatistics) MergeSnapshot(snapshot StatisticsSnapshot) MergeResult {
	result := MergeResult{}
	if s == nil {
		return result
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	seen := s.buildSeenDetailsLocked()
	if snapshotHasAggregateFields(snapshot) {
		s.mergeAggregateSnapshotLocked(snapshot, seen, &result)
		return result
	}

	s.mergeLegacySnapshotLocked(snapshot, seen, &result)
	return result
}

func (s *RequestStatistics) mergeAggregateSnapshotLocked(
	snapshot StatisticsSnapshot,
	seen map[string]struct{},
	result *MergeResult,
) {
	s.totalRequests += snapshot.TotalRequests
	s.successCount += snapshot.SuccessCount
	s.failureCount += snapshot.FailureCount
	s.totalTokens += snapshot.TotalTokens
	s.tokenStats = addTokenStats(s.tokenStats, normaliseTokenStats(snapshot.TokenStats))
	s.maxRequestsPerMinute = maxInt64(s.maxRequestsPerMinute, snapshot.MaxRequestsPerMinute)
	s.maxTokensPerMinute = maxInt64(s.maxTokensPerMinute, snapshot.MaxTokensPerMinute)

	mergeInt64Map(s.requestsByDay, snapshot.RequestsByDay)
	mergeLegacyHourMap(s.requestsByHour, snapshot.RequestsByHour)
	mergeInt64Map(s.tokensByDay, snapshot.TokensByDay)
	mergeLegacyHourMap(s.tokensByHour, snapshot.TokensByHour)
	mergeInt64Map(s.recentRequestsByMinute, snapshot.RecentRequestsByMinute)
	mergeInt64Map(s.recentTokensByMinute, snapshot.RecentTokensByMinute)
	mergeServiceHealthMap(s.serviceHealthByBucket, snapshot.ServiceHealthByBucket)
	s.refreshPerMinutePeaksLocked()
	s.pruneRecentMapsLocked(time.Now().UTC())

	for apiName, apiSnapshot := range snapshot.APIs {
		apiName = strings.TrimSpace(apiName)
		if apiName == "" {
			continue
		}
		apiEntry := s.ensureAPIStatsLocked(apiName)
		if apiEntry.Models == nil {
			apiEntry.Models = make(map[string]*modelStats)
		}
		for modelName, modelSnapshot := range apiSnapshot.Models {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				modelName = "unknown"
			}
			modelEntry := s.ensureModelStatsLocked(apiEntry, modelName)
			summary := summarizeSnapshotModel(modelSnapshot)

			modelEntry.TotalRequests += summary.totalRequests
			modelEntry.SuccessCount += summary.successCount
			modelEntry.FailureCount += summary.failureCount
			modelEntry.TotalTokens += summary.totalTokens
			modelEntry.TokenStats = addTokenStats(modelEntry.TokenStats, summary.tokenStats)

			apiEntry.TotalRequests += summary.totalRequests
			apiEntry.SuccessCount += summary.successCount
			apiEntry.FailureCount += summary.failureCount
			apiEntry.TotalTokens += summary.totalTokens
			apiEntry.TokenStats = addTokenStats(apiEntry.TokenStats, summary.tokenStats)

			mergeInt64Map(modelEntry.RequestsByDay, modelSnapshot.RequestsByDay)
			mergeInt64Map(modelEntry.RequestsByHour, modelSnapshot.RequestsByHour)
			mergeInt64Map(modelEntry.TokensByDay, modelSnapshot.TokensByDay)
			mergeInt64Map(modelEntry.TokensByHour, modelSnapshot.TokensByHour)
			mergeTokenStatsMap(modelEntry.TokenStatsByDay, modelSnapshot.TokenStatsByDay)
			mergeTokenStatsMap(modelEntry.TokenStatsByHour, modelSnapshot.TokenStatsByHour)
			pruneRecentInt64Map(modelEntry.RequestsByHour, recentHourBucketLimit)
			pruneRecentInt64Map(modelEntry.TokensByHour, recentHourBucketLimit)
			pruneRecentTokenStatsMap(modelEntry.TokenStatsByHour, recentHourBucketLimit)

			for _, detail := range modelSnapshot.Details {
				detail.Tokens = normaliseTokenStats(detail.Tokens)
				if detail.LatencyMs < 0 {
					detail.LatencyMs = 0
				}
				if detail.Timestamp.IsZero() {
					detail.Timestamp = time.Now().UTC()
				} else {
					detail.Timestamp = detail.Timestamp.UTC()
				}
				key := dedupKey(apiName, modelName, detail)
				if _, exists := seen[key]; exists {
					result.Skipped++
					continue
				}
				seen[key] = struct{}{}
				s.appendDetailLocked(apiName, modelName, modelEntry, detail)
				result.Added++
			}
		}
	}
}

func (s *RequestStatistics) mergeLegacySnapshotLocked(
	snapshot StatisticsSnapshot,
	seen map[string]struct{},
	result *MergeResult,
) {
	for apiName, apiSnapshot := range snapshot.APIs {
		apiName = strings.TrimSpace(apiName)
		if apiName == "" {
			continue
		}
		apiEntry := s.ensureAPIStatsLocked(apiName)
		for modelName, modelSnapshot := range apiSnapshot.Models {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				modelName = "unknown"
			}
			for _, detail := range modelSnapshot.Details {
				detail.Tokens = normaliseTokenStats(detail.Tokens)
				if detail.LatencyMs < 0 {
					detail.LatencyMs = 0
				}
				if detail.Timestamp.IsZero() {
					detail.Timestamp = time.Now().UTC()
				} else {
					detail.Timestamp = detail.Timestamp.UTC()
				}
				key := dedupKey(apiName, modelName, detail)
				if _, exists := seen[key]; exists {
					result.Skipped++
					continue
				}
				seen[key] = struct{}{}
				s.recordImportedLegacyLocked(apiName, modelName, apiEntry, detail)
				result.Added++
			}
		}
	}
}

func (s *RequestStatistics) recordImportedLegacyLocked(apiName, modelName string, apiEntry *apiStats, detail RequestDetail) {
	success := !detail.Failed
	totalTokens := detail.Tokens.TotalTokens

	s.totalRequests++
	if success {
		s.successCount++
	} else {
		s.failureCount++
	}
	s.totalTokens += totalTokens
	s.tokenStats = addTokenStats(s.tokenStats, detail.Tokens)

	modelEntry := s.ensureModelStatsLocked(apiEntry, modelName)
	s.applyAggregateToAPIAndModelLocked(apiEntry, modelEntry, detail.Tokens, success)
	s.appendDetailLocked(apiName, modelName, modelEntry, detail)
	s.updateTopLevelBucketsLocked(detail.Timestamp.UTC(), detail.Tokens, success)
	s.updateModelBucketsLocked(modelEntry, detail.Timestamp.UTC(), detail.Tokens)
}

func (s *RequestStatistics) buildSeenDetailsLocked() map[string]struct{} {
	seen := make(map[string]struct{})
	for apiName, stats := range s.apis {
		if stats == nil {
			continue
		}
		for modelName, modelStatsValue := range stats.Models {
			if modelStatsValue == nil {
				continue
			}
			for _, detail := range modelStatsValue.Details {
				seen[dedupKey(apiName, modelName, detail)] = struct{}{}
			}
		}
	}
	return seen
}

func (s *RequestStatistics) ensureAPIStatsLocked(apiName string) *apiStats {
	stats, ok := s.apis[apiName]
	if ok && stats != nil {
		if stats.Models == nil {
			stats.Models = make(map[string]*modelStats)
		}
		return stats
	}
	stats = &apiStats{Models: make(map[string]*modelStats)}
	s.apis[apiName] = stats
	return stats
}

func (s *RequestStatistics) ensureModelStatsLocked(apiEntry *apiStats, modelName string) *modelStats {
	if apiEntry.Models == nil {
		apiEntry.Models = make(map[string]*modelStats)
	}
	stats, ok := apiEntry.Models[modelName]
	if ok && stats != nil {
		s.ensureModelMapsLocked(stats)
		return stats
	}
	stats = &modelStats{}
	s.ensureModelMapsLocked(stats)
	apiEntry.Models[modelName] = stats
	return stats
}

func (s *RequestStatistics) ensureModelMapsLocked(stats *modelStats) {
	if stats.RequestsByDay == nil {
		stats.RequestsByDay = make(map[string]int64)
	}
	if stats.RequestsByHour == nil {
		stats.RequestsByHour = make(map[string]int64)
	}
	if stats.TokensByDay == nil {
		stats.TokensByDay = make(map[string]int64)
	}
	if stats.TokensByHour == nil {
		stats.TokensByHour = make(map[string]int64)
	}
	if stats.TokenStatsByDay == nil {
		stats.TokenStatsByDay = make(map[string]TokenStats)
	}
	if stats.TokenStatsByHour == nil {
		stats.TokenStatsByHour = make(map[string]TokenStats)
	}
}

func (s *RequestStatistics) applyAggregateToAPIAndModelLocked(
	apiEntry *apiStats,
	modelEntry *modelStats,
	tokens TokenStats,
	success bool,
) {
	apiEntry.TotalRequests++
	if success {
		apiEntry.SuccessCount++
	} else {
		apiEntry.FailureCount++
	}
	apiEntry.TotalTokens += tokens.TotalTokens
	apiEntry.TokenStats = addTokenStats(apiEntry.TokenStats, tokens)

	modelEntry.TotalRequests++
	if success {
		modelEntry.SuccessCount++
	} else {
		modelEntry.FailureCount++
	}
	modelEntry.TotalTokens += tokens.TotalTokens
	modelEntry.TokenStats = addTokenStats(modelEntry.TokenStats, tokens)
}

func (s *RequestStatistics) appendDetailLocked(
	apiName, modelName string,
	modelEntry *modelStats,
	detail RequestDetail,
) {
	s.nextSequence++
	detail.Sequence = s.nextSequence
	limit := RetainedRequestDetailsLimit()
	if limit <= 0 {
		modelEntry.Details = nil
		s.detailOrder = nil
		return
	}
	modelEntry.Details = append(modelEntry.Details, detail)
	s.detailOrder = append(s.detailOrder, detailRef{
		apiName:   apiName,
		modelName: modelName,
		sequence:  detail.Sequence,
	})
	for len(s.detailOrder) > limit {
		s.evictOldestDetailLocked()
	}
}

func (s *RequestStatistics) evictOldestDetailLocked() {
	if len(s.detailOrder) == 0 {
		return
	}
	ref := s.detailOrder[0]
	s.detailOrder = s.detailOrder[1:]

	apiEntry := s.apis[ref.apiName]
	if apiEntry == nil {
		return
	}
	modelEntry := apiEntry.Models[ref.modelName]
	if modelEntry == nil {
		return
	}
	for idx, detail := range modelEntry.Details {
		if detail.Sequence != ref.sequence {
			continue
		}
		modelEntry.Details = append(modelEntry.Details[:idx], modelEntry.Details[idx+1:]...)
		return
	}
}

func (s *RequestStatistics) updateTopLevelBucketsLocked(timestamp time.Time, tokens TokenStats, success bool) {
	dayKey := formatDayBucket(timestamp)
	hourOfDay := timestamp.Hour()
	minuteKey := formatMinuteBucket(timestamp)
	quarterHourKey := formatQuarterHourBucket(timestamp)

	s.requestsByDay[dayKey]++
	s.requestsByHour[hourOfDay]++
	s.tokensByDay[dayKey] += tokens.TotalTokens
	s.tokensByHour[hourOfDay] += tokens.TotalTokens

	nowUTC := time.Now().UTC()
	if timestamp.After(nowUTC.Add(time.Minute)) {
		return
	}
	cutoffMinute := nowUTC.Add(-(recentMinuteBucketLimit - 1) * time.Minute)
	if !timestamp.Before(cutoffMinute) {
		s.recentRequestsByMinute[minuteKey]++
		s.recentTokensByMinute[minuteKey] += tokens.TotalTokens
		s.maxRequestsPerMinute = maxInt64(s.maxRequestsPerMinute, s.recentRequestsByMinute[minuteKey])
		s.maxTokensPerMinute = maxInt64(s.maxTokensPerMinute, s.recentTokensByMinute[minuteKey])
	}

	cutoffQuarter := nowUTC.Add(-time.Duration(serviceHealthBucketLimit-1) * 15 * time.Minute)
	if !timestamp.Before(cutoffQuarter) {
		bucket := s.serviceHealthByBucket[quarterHourKey]
		if success {
			bucket.SuccessCount++
		} else {
			bucket.FailureCount++
		}
		s.serviceHealthByBucket[quarterHourKey] = bucket
	}

	s.pruneRecentMapsLocked(nowUTC)
}

func (s *RequestStatistics) updateModelBucketsLocked(modelEntry *modelStats, timestamp time.Time, tokens TokenStats) {
	dayKey := formatDayBucket(timestamp)
	hourKey := formatHourBucket(timestamp)

	modelEntry.RequestsByDay[dayKey]++
	modelEntry.RequestsByHour[hourKey]++
	modelEntry.TokensByDay[dayKey] += tokens.TotalTokens
	modelEntry.TokensByHour[hourKey] += tokens.TotalTokens
	modelEntry.TokenStatsByDay[dayKey] = addTokenStats(modelEntry.TokenStatsByDay[dayKey], tokens)
	modelEntry.TokenStatsByHour[hourKey] = addTokenStats(modelEntry.TokenStatsByHour[hourKey], tokens)

	pruneRecentInt64Map(modelEntry.RequestsByHour, recentHourBucketLimit)
	pruneRecentInt64Map(modelEntry.TokensByHour, recentHourBucketLimit)
	pruneRecentTokenStatsMap(modelEntry.TokenStatsByHour, recentHourBucketLimit)
}

func (s *RequestStatistics) pruneRecentMapsLocked(nowUTC time.Time) {
	cutoffMinute := formatMinuteBucket(nowUTC.Add(-(recentMinuteBucketLimit - 1) * time.Minute))
	for key := range s.recentRequestsByMinute {
		if key < cutoffMinute {
			delete(s.recentRequestsByMinute, key)
		}
	}
	for key := range s.recentTokensByMinute {
		if key < cutoffMinute {
			delete(s.recentTokensByMinute, key)
		}
	}

	cutoffQuarter := formatQuarterHourBucket(nowUTC.Add(-time.Duration(serviceHealthBucketLimit-1) * 15 * time.Minute))
	for key := range s.serviceHealthByBucket {
		if key < cutoffQuarter {
			delete(s.serviceHealthByBucket, key)
		}
	}
}

type snapshotModelSummary struct {
	totalRequests int64
	successCount  int64
	failureCount  int64
	totalTokens   int64
	tokenStats    TokenStats
}

func summarizeSnapshotModel(snapshot ModelSnapshot) snapshotModelSummary {
	tokenStats := normaliseTokenStats(snapshot.TokenStats)
	totalRequests := snapshot.TotalRequests
	successCount := snapshot.SuccessCount
	failureCount := snapshot.FailureCount
	totalTokens := snapshot.TotalTokens

	if totalRequests == 0 && len(snapshot.Details) > 0 {
		totalRequests = int64(len(snapshot.Details))
	}
	if successCount == 0 && failureCount == 0 && len(snapshot.Details) > 0 {
		for _, detail := range snapshot.Details {
			if detail.Failed {
				failureCount++
			} else {
				successCount++
			}
		}
	}
	if totalTokens == 0 {
		totalTokens = tokenStats.TotalTokens
	}
	if isZeroTokenStats(tokenStats) && len(snapshot.Details) > 0 {
		for _, detail := range snapshot.Details {
			tokenStats = addTokenStats(tokenStats, normaliseTokenStats(detail.Tokens))
		}
		totalTokens = tokenStats.TotalTokens
	}

	return snapshotModelSummary{
		totalRequests: totalRequests,
		successCount:  successCount,
		failureCount:  failureCount,
		totalTokens:   totalTokens,
		tokenStats:    tokenStats,
	}
}

func snapshotHasAggregateFields(snapshot StatisticsSnapshot) bool {
	if snapshot.MaxRequestsPerMinute != 0 ||
		snapshot.MaxTokensPerMinute != 0 ||
		!isZeroTokenStats(snapshot.TokenStats) ||
		len(snapshot.RecentRequestsByMinute) > 0 ||
		len(snapshot.RecentTokensByMinute) > 0 ||
		len(snapshot.ServiceHealthByBucket) > 0 {
		return true
	}
	for _, apiSnapshot := range snapshot.APIs {
		if apiSnapshot.SuccessCount != 0 || apiSnapshot.FailureCount != 0 || !isZeroTokenStats(apiSnapshot.TokenStats) {
			return true
		}
		for _, modelSnapshot := range apiSnapshot.Models {
			if modelSnapshot.SuccessCount != 0 ||
				modelSnapshot.FailureCount != 0 ||
				!isZeroTokenStats(modelSnapshot.TokenStats) ||
				len(modelSnapshot.RequestsByDay) > 0 ||
				len(modelSnapshot.RequestsByHour) > 0 ||
				len(modelSnapshot.TokensByDay) > 0 ||
				len(modelSnapshot.TokensByHour) > 0 ||
				len(modelSnapshot.TokenStatsByDay) > 0 ||
				len(modelSnapshot.TokenStatsByHour) > 0 {
				return true
			}
		}
	}
	return false
}

func dedupKey(apiName, modelName string, detail RequestDetail) string {
	timestamp := detail.Timestamp.UTC().Format(time.RFC3339Nano)
	tokens := normaliseTokenStats(detail.Tokens)
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%t|%d|%d|%d|%d|%d",
		apiName,
		modelName,
		timestamp,
		detail.Source,
		detail.AuthIndex,
		detail.Failed,
		tokens.InputTokens,
		tokens.OutputTokens,
		tokens.ReasoningTokens,
		tokens.CachedTokens,
		tokens.TotalTokens,
	)
}

func maxInt64(values ...int64) int64 {
	var maxValue int64
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func (s *RequestStatistics) refreshPerMinutePeaksLocked() {
	s.maxRequestsPerMinute = maxInt64(s.maxRequestsPerMinute, maxValueInMap(s.recentRequestsByMinute))
	s.maxTokensPerMinute = maxInt64(s.maxTokensPerMinute, maxValueInMap(s.recentTokensByMinute))
}

func maxValueInMap(values map[string]int64) int64 {
	var maxValue int64
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func resolveAPIIdentifier(ctx context.Context, record coreusage.Record) string {
	if ctx != nil {
		if ginCtx, ok := ctx.Value("gin").(*gin.Context); ok && ginCtx != nil {
			path := ginCtx.FullPath()
			if path == "" && ginCtx.Request != nil {
				path = ginCtx.Request.URL.Path
			}
			method := ""
			if ginCtx.Request != nil {
				method = ginCtx.Request.Method
			}
			if path != "" {
				if method != "" {
					return method + " " + path
				}
				return path
			}
		}
	}
	if record.Provider != "" {
		return record.Provider
	}
	return "unknown"
}

func resolveSuccess(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	ginCtx, ok := ctx.Value("gin").(*gin.Context)
	if !ok || ginCtx == nil {
		return true
	}
	status := ginCtx.Writer.Status()
	if status == 0 {
		return true
	}
	return status < httpStatusBadRequest
}

const httpStatusBadRequest = 400

func normaliseDetail(detail coreusage.Detail) TokenStats {
	tokens := TokenStats{
		InputTokens:     detail.InputTokens,
		OutputTokens:    detail.OutputTokens,
		ReasoningTokens: detail.ReasoningTokens,
		CachedTokens:    detail.CachedTokens,
		TotalTokens:     detail.TotalTokens,
	}
	return normaliseTokenStats(tokens)
}

func normaliseTokenStats(tokens TokenStats) TokenStats {
	if tokens.InputTokens < 0 {
		tokens.InputTokens = 0
	}
	if tokens.OutputTokens < 0 {
		tokens.OutputTokens = 0
	}
	if tokens.ReasoningTokens < 0 {
		tokens.ReasoningTokens = 0
	}
	if tokens.CachedTokens < 0 {
		tokens.CachedTokens = 0
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens + tokens.ReasoningTokens
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens + tokens.ReasoningTokens + tokens.CachedTokens
	}
	if tokens.TotalTokens < 0 {
		tokens.TotalTokens = 0
	}
	return tokens
}

func addTokenStats(left, right TokenStats) TokenStats {
	left = normaliseTokenStats(left)
	right = normaliseTokenStats(right)
	return normaliseTokenStats(TokenStats{
		InputTokens:     left.InputTokens + right.InputTokens,
		OutputTokens:    left.OutputTokens + right.OutputTokens,
		ReasoningTokens: left.ReasoningTokens + right.ReasoningTokens,
		CachedTokens:    left.CachedTokens + right.CachedTokens,
		TotalTokens:     left.TotalTokens + right.TotalTokens,
	})
}

func isZeroTokenStats(tokens TokenStats) bool {
	return tokens.InputTokens == 0 &&
		tokens.OutputTokens == 0 &&
		tokens.ReasoningTokens == 0 &&
		tokens.CachedTokens == 0 &&
		tokens.TotalTokens == 0
}

func normaliseLatency(latency time.Duration) int64 {
	if latency <= 0 {
		return 0
	}
	return latency.Milliseconds()
}

func formatLegacyHour(hour int) string {
	if hour < 0 {
		hour = 0
	}
	hour = hour % 24
	return fmt.Sprintf("%02d", hour)
}

func formatDayBucket(timestamp time.Time) string {
	return timestamp.UTC().Format("2006-01-02")
}

func formatHourBucket(timestamp time.Time) string {
	return timestamp.UTC().Truncate(time.Hour).Format(time.RFC3339)
}

func formatMinuteBucket(timestamp time.Time) string {
	return timestamp.UTC().Truncate(time.Minute).Format(time.RFC3339)
}

func formatQuarterHourBucket(timestamp time.Time) string {
	minutes := timestamp.UTC().Minute()
	truncated := timestamp.UTC().Truncate(time.Hour).Add(time.Duration(minutes/15*15) * time.Minute)
	return truncated.Format(time.RFC3339)
}

func cloneInt64Map(source map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneTokenStatsMap(source map[string]TokenStats) map[string]TokenStats {
	result := make(map[string]TokenStats, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneServiceHealthMap(source map[string]ServiceHealthSnapshot) map[string]ServiceHealthSnapshot {
	result := make(map[string]ServiceHealthSnapshot, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func mergeInt64Map(target, source map[string]int64) {
	for key, value := range source {
		target[key] += value
	}
}

func mergeLegacyHourMap(target map[int]int64, source map[string]int64) {
	for key, value := range source {
		if parsedHour, ok := parseLegacyHour(key); ok {
			target[parsedHour] += value
		}
	}
}

func parseLegacyHour(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if len(value) != 2 {
		return 0, false
	}
	var hour int
	if _, err := fmt.Sscanf(value, "%02d", &hour); err != nil {
		return 0, false
	}
	if hour < 0 || hour > 23 {
		return 0, false
	}
	return hour, true
}

func mergeTokenStatsMap(target, source map[string]TokenStats) {
	for key, value := range source {
		target[key] = addTokenStats(target[key], value)
	}
}

func mergeServiceHealthMap(target, source map[string]ServiceHealthSnapshot) {
	for key, value := range source {
		current := target[key]
		current.SuccessCount += value.SuccessCount
		current.FailureCount += value.FailureCount
		target[key] = current
	}
}

func pruneRecentInt64Map(values map[string]int64, keep int) {
	if keep <= 0 || len(values) <= keep {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys[:len(keys)-keep] {
		delete(values, key)
	}
}

func pruneRecentTokenStatsMap(values map[string]TokenStats, keep int) {
	if keep <= 0 || len(values) <= keep {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys[:len(keys)-keep] {
		delete(values, key)
	}
}
