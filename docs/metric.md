# Metric Requirements

This document defines the repository's required behavior for usage metrics, usage detail retention, and frontend metric rendering.

## Goals

- Keep long-term metric accuracy even under high request volume.
- Prevent unbounded memory growth from per-request usage details.
- Ensure dashboards, health views, and trend charts do not regress when request detail retention is capped.

## Required Behavior

### 1. Aggregates Must Be Long-Lived

- Usage statistics must retain aggregate values independently of per-request details.
- The following aggregate values must remain accurate for the full retained usage snapshot:
  - total requests
  - successful requests
  - failed requests
  - total tokens
  - token breakdowns when available
  - RPM/TPM source buckets
  - peak requests-per-minute and peak tokens-per-minute metrics
  - cost-related aggregate inputs
  - service health buckets
  - request and token trend buckets
  - per-endpoint aggregate stats
  - per-model aggregate stats

### 2. Per-Request Details Are Bounded

- Per-request usage details must not grow without bound in memory.
- Only the most recent N request details are retained.
- Default N is `500`.
- The retention limit is configured by `usage-statistics-max-details`.
- `usage-statistics-max-details: 0` means:
  - keep no per-request details
  - keep aggregate statistics only
- Negative values must fall back to the default behavior rather than creating invalid retention state.

### 3. Aggregates Must Not Depend On Full Detail History

- Total requests, total tokens, RPM, TPM, total cost, service health, request trend, token trend, token breakdown trend, and model/API statistics must not rely on having the full request detail history in memory.
- Capping detail retention must not change the meaning of aggregate metrics.
- Any feature that represents a long-running summary must read aggregate counters or aggregate time buckets, not just `details`.

### 4. Frontend Rendering Rules

- Summary cards must prefer aggregate fields over recomputing from retained `details`.
- RPM and TPM must preserve the intended time-range semantics.
- Peak requests-per-minute and peak tokens-per-minute must preserve the intended time-range semantics.
- Service health views must remain correct when detail retention is capped.
- Request and usage trend charts must remain correct when detail retention is capped.
- Cost trend and token breakdown trend must remain correct when detail retention is capped.
- Event/detail tables may use retained request details and therefore only show the recent window.

### 5. Export / Import Semantics

- Usage snapshot export must include the aggregate values needed to preserve long-term metrics.
- Import must preserve those aggregates even if exported `details` are capped.
- Importing a snapshot must not reduce aggregate totals simply because only a bounded number of `details` are present.

### 6. Backward Compatibility

- Older snapshots that only contain detail-based usage data should still import successfully.
- When explicit aggregate fields are absent, the system may derive values from available details as a fallback.

## Implementation Guidance

- Treat `details` as a recent-event buffer.
- Treat aggregate counters and time buckets as the source of truth for long-term reporting.
- When adding a new metric-oriented UI or API field:
  - decide whether it is a recent-event feature or a long-term aggregate feature
  - use aggregate storage for long-term metrics
  - avoid introducing any requirement for unbounded in-memory detail retention

## Configuration

- `usage-statistics-enabled`
  - enables in-memory usage aggregation
- `usage-statistics-persist`
  - persists usage snapshots across restarts
- `usage-statistics-flush-interval-seconds`
  - controls persistence flush timing
- `usage-statistics-max-details`
  - controls recent detail retention
  - default: `500`
  - `0`: keep aggregates only

## Change Safety Checklist

- If you touch backend usage aggregation:
  - verify totals remain correct after detail eviction
  - verify snapshot export/import preserves aggregates
  - verify service health and trend buckets remain populated
- If you touch frontend usage pages:
  - verify total requests/tokens are not derived only from retained details
  - verify RPM/TPM still follow the selected time range
  - verify per-minute peak metrics still follow the selected time range
  - verify service health still works with capped details
  - verify request/token/cost/token-breakdown charts still work with capped details
