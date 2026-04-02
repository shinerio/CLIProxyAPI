# Repository Guidelines

## Project Structure & Module Organization
This workspace is split into two projects. `backend/` is the Go service: `cmd/server` is the main entrypoint, `internal/` holds application code, `sdk/` exposes reusable packages, `examples/` contains sample integrations, and `test/` is for cross-package coverage. `frontend/` is the management UI built with Vite + React + TypeScript; main code lives in `frontend/src/` with feature code in `pages/`, `components/`, `services/api/`, `stores/`, and `i18n/locales/`.

## Metrics Guidance
For any change that affects usage metrics, usage retention, dashboard totals, RPM/TPM, service health, or trend charts, follow `docs/metric.md` as the source of truth for required behavior and compatibility constraints.

## Build, Test, and Development Commands
- `cd backend && go run ./cmd/server -config ./config.yaml`: run the API locally.
- `cd backend && go build ./cmd/server`: build the backend binary.
- `cd backend && go test ./...`: run backend unit and integration tests.
- `cd frontend && npm ci`: install locked frontend dependencies.
- `cd frontend && npm run dev`: start the Vite dev server on `http://localhost:5173`.
- `cd frontend && npm run build`: run `tsc` and emit the single-file production build.
- `cd frontend && npm run lint && npm run type-check`: run the minimum frontend verification gate.

## Coding Style & Naming Conventions
Go code should stay `gofmt`-clean, with lowercase package names, exported `PascalCase` identifiers, and colocated `*_test.go` files. Frontend formatting follows `frontend/.prettierrc`: 2-space indentation, semicolons, single quotes, trailing commas, and 100-character lines. Match existing names such as `useAuthStore.ts`, `DashboardPage.tsx`, `*_handler.go`, and `*_executor.go`.

## Testing Guidelines
Add regression tests for backend fixes and prefer table-driven tests when behavior varies by provider, route, or model family. Frontend currently relies on static checks plus manual verification, so run `npm run lint`, `npm run type-check`, and exercise the affected screen in `npm run dev`. Keep test files beside the code they cover unless the scenario spans packages.

## Commit & Pull Request Guidelines
Existing project guidance favors short, imperative commit subjects. Backend conventions often use Conventional Commit prefixes such as `feat(auth): ...` or `fix(claude): ...`; frontend commits should also read cleanly in release notes, for example `fix quota card refresh`. PRs should summarize the change, note config or API impact, link the issue when available, and include screenshots for UI work plus the commands you ran.

## Security & Configuration Tips
Never commit real API keys, OAuth tokens, or populated `.env` files. Start from `backend/config.example.yaml` and `backend/.env.example`, keep local fixtures sanitized, and avoid committing generated output from `frontend/dist/` or dependencies from `node_modules/`.
