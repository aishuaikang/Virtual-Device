# Repository Guidelines

## Project Structure & Module Organization
This Wails desktop app combines a Go backend with a React + TypeScript frontend. `main.go` and `app.go` bootstrap Wails and expose backend methods to the UI. Backend code lives in `internal/`: `engine/` handles runtime orchestration, `modules/` contains feature logic, `config/` stores app settings, `scenes/` manages saved presets, `protocol/` defines shared protocol types, `mocks/` provides simulated devices, and `utils/` holds reusable helpers and unit tests. Frontend code lives in `frontend/src`, with reusable UI in `frontend/src/components`, localization files in `frontend/src/i18n`, and generated Wails bindings in `frontend/wailsjs`. Build outputs and packaging assets live in `build/`.

## Build, Test, and Development Commands
- `wails dev` — run the desktop app with live frontend reload.
- `wails build` — create a production desktop bundle.
- `go test ./...` — run all Go tests, including `internal/utils` and `internal/scenes`.
- `cd frontend && bun run dev` — start the Vite frontend in isolation.
- `cd frontend && bun run build` — type-check and bundle the frontend.

## Coding Style & Naming Conventions
Format Go code with `gofmt -w` and follow standard Go conventions: tabs for indentation, lowercase package names, `PascalCase` for exported identifiers, and `camelCase` for unexported names. Keep tests in `*_test.go`. In the frontend, use strict TypeScript, functional React components, `PascalCase` filenames such as `ConfigEditor.tsx`, and `camelCase` for props, hooks, and helpers. Keep Wails bridge calls close to the UI boundary.

## Testing Guidelines
Use Go’s built-in `testing` package and prefer table-driven tests for backend logic. Place tests beside the code they cover and name them `TestXxx`. Run `go test ./...` before submitting changes. There is no frontend test runner configured yet, so validate UI work manually with `wails dev`.

## Commit & Pull Request Guidelines
This checkout does not include `.git` history, so follow a simple convention: short, imperative commit subjects such as `Add scene rename validation`, with one logical change per commit. Pull requests should describe the user-visible impact, note backend and frontend files touched, list verification commands, and include screenshots or recordings for UI updates.

## Configuration Notes
`wails.json` delegates frontend install and build steps to Bun. When exported Go APIs change, regenerate and commit the matching files under `frontend/wailsjs`, and keep frontend dependency manifests in sync.
