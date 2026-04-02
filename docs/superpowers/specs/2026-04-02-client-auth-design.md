# Client Authentication Page Design

## Summary

The current authentication configuration lives inside the visual configuration editor on the config panel. It is mixed with unrelated system settings, which makes authentication-specific browsing and management slower once the number of client API keys grows.

This change extracts authentication configuration into a dedicated top-level page named `Client Authentication`, placed directly below `Config Panel` in the main sidebar. The new page manages only:

- `auth-dir`
- `api-keys`

Remote management settings remain on the config panel and are explicitly out of scope.

## Goals

- Make authentication configuration discoverable from the main navigation.
- Improve browsing of existing client API keys before editing.
- Keep the save path stable by reusing the existing YAML fetch, merge, diff, and save flow.
- Remove duplicate entry points for the same authentication settings.

## Non-Goals

- Changing backend APIs or introducing a separate authentication config endpoint.
- Moving `remote-management` settings, including `secret-key`.
- Redesigning unrelated config panel sections.
- Changing auth file management or OAuth management information architecture.

## Current State

- Main navigation exposes `Config Panel` but not a dedicated client authentication entry.
- `ConfigPage` owns config YAML loading, diffing, and saving.
- `VisualConfigEditor` includes an `auth` section containing `auth-dir` and `api-keys`.
- `ApiKeysCardEditor` already supports add, edit, delete, copy, key generation, and restricting access by credential scope.

## Chosen Approach

Create a new top-level frontend route, `/client-auth`, and a dedicated page that reuses the existing config YAML workflow and API key editing componentry.

This approach was chosen because it cleanly separates navigation concerns without requiring backend changes, while preserving the existing save semantics that already protect against overwriting unrelated YAML content.

## Rejected Alternatives

### Keep everything under `/config` with a secondary menu

Rejected because the user explicitly wants `Client Authentication` as a standalone main-sidebar item below `Config Panel`.

### Add `/client-auth` but keep the auth section inside `ConfigPage`

Rejected because two entry points for the same settings would create ambiguity and increase the chance of divergent UX and future maintenance issues.

### Build a new backend endpoint for partial auth config saves

Rejected because it expands scope unnecessarily and duplicates logic that is already working through the YAML merge-and-diff path.

## Information Architecture

### Main Sidebar

Update main navigation order to:

1. Dashboard
2. Config Panel
3. Client Authentication
4. AI Providers
5. Auth Files
6. OAuth
7. Quota
8. Usage
9. Logs (when enabled)
10. System

### Route Structure

- Add `/client-auth`
- Keep `/config`
- Remove the authentication section from `ConfigPage`

### Ownership Boundaries

`/client-auth` owns:

- `auth-dir`
- `api-keys`

`/config` continues to own:

- server
- TLS
- remote management
- system
- network
- quota
- streaming
- payload

## Page Design

### Header

The new page header should clearly establish that this screen manages client-facing authentication only.

Header content:

- Title: `Client Authentication`
- Description: manage client access directory and client API keys
- Status badge derived from connection state
- Primary actions:
  - refresh
  - add client key
  - save changes

### Desktop Layout

Use a two-column workspace:

- Left column: sticky summary rail
- Right column: primary browsing and editing workspace

Recommended proportions:

- left: about 320px
- right: flexible remaining width

### Left Summary Rail

The rail should optimize orientation and quick context, not duplicate the full editor.

Sections:

- `auth-dir` card
  - editable input
  - short explanation of what directory it controls
- summary card
  - total client key count
  - count of restricted keys
  - count of unrestricted keys
- page-level help text
  - clarify that remote management settings are not edited here

### Right Workspace

The right side should optimize browsing before editing.

Sections:

- toolbar
  - keyword search
  - scope filter: all, unrestricted, restricted
  - sort: default, name
- key browser
  - card-based list of client keys
  - each card shows:
    - display name
    - masked API key
    - scope summary
    - count of allowed credentials when restricted
  - card actions:
    - copy
    - edit
    - delete

### Add/Edit Modal

Reuse the current modal interaction model, but improve browsing inside the credential scope chooser.

Enhancements:

- keep key generation
- keep validation
- keep allowed credential selection
- add credential search inside the modal
- keep empty selection semantics as "allow all credentials"

### Mobile Layout

On mobile, stack sections vertically:

- header actions remain available
- `auth-dir` summary card first
- client key browser below
- save affordance remains easy to reach, either in the header area or the existing floating action pattern

## Data Flow and Save Behavior

The new page must reuse the existing config file lifecycle:

1. fetch current YAML from the server
2. parse into visual auth values
3. edit only `auth-dir` and `api-keys`
4. on save, refetch latest YAML
5. merge only the owned fields into that latest YAML
6. show the existing diff confirmation
7. persist through the existing config save API
8. refresh global config store after save

This keeps one source of truth and avoids introducing partial-save divergence.

## Component Changes

### New Page

Add a dedicated page component for client authentication. It should own:

- YAML loading state
- dirty state
- page-local search and filter state
- save and diff flow

### Shared Auth Editor Extraction

Extract the auth-specific visual editing pieces from the current config editor into reusable building blocks. The config page will stop rendering them, while the new page will render them in the new layout.

Likely extraction targets:

- `auth-dir` editor block
- `ApiKeysCardEditor`
- lightweight auth stats helpers for filtered browsing

### Main Navigation

Add the new sidebar item and include it in route ordering and transition logic.

### Config Page

Remove the old auth section so the config panel no longer exposes `auth-dir` or `api-keys`.

## UX Rules

- `Client Authentication` is the only place where client auth config is edited.
- `remote-management.secret-key` is never shown on this page.
- Search filters the visible client key cards only; it does not mutate data.
- Empty scope means unrestricted access to all available credentials.
- Saving from `/client-auth` must not rewrite unrelated config sections beyond the existing normalization already performed by the current diff/save flow.

## Error Handling

- If YAML cannot be parsed into visual auth values, fall back to a source-oriented recovery message rather than silently dropping fields.
- Save failures continue to use existing notifications.
- If credential metadata cannot be loaded for scope browsing, the API key editor remains usable and shows a degraded empty or partial state.

## Testing Plan

### Static and Functional Checks

- verify new route renders from sidebar navigation
- verify sidebar ordering and active-state behavior
- verify config page no longer shows the auth section
- verify client auth page loads existing `auth-dir` and `api-keys`
- verify add, edit, copy, delete, and generate key flows
- verify restricted and unrestricted scope display
- verify search, filter, and sort behavior
- verify diff modal shows auth-only changes when applicable
- verify save refreshes shared config state

### Regression Checks

- verify remote management controls remain on `/config`
- verify `/client-auth` does not expose remote management fields
- verify navigation transitions remain correct for `/config`, `/client-auth`, and `/auth-files`
- verify mobile layout remains usable

## Implementation Notes

- Prefer reusing the existing `useVisualConfig` parsing and YAML application logic instead of duplicating auth-specific YAML handling.
- Keep the new page visually aligned with the current config panel language, but make the content denser and more browseable.
- Avoid introducing a second source mode editor unless a recovery path is required by existing parse constraints.

## Open Questions Resolved

- Move remote management too: no
- New entry location: main sidebar directly below config panel
- Scope of the new page: authentication configuration only

## Rollout Result

After this change:

- users find authentication settings directly from the main sidebar
- config panel focuses on non-auth system configuration
- client API keys become easier to scan, search, and manage
