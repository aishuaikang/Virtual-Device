# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Desktop app (Go + frontend together)
wails dev          # Live dev mode — hot-reloads frontend, exposes Go methods at http://localhost:34115
wails build        # Production build (embeds frontend/dist into binary)

# Frontend only (inside frontend/)
bun install        # Install JS dependencies
bun run dev        # Vite dev server
bun run build      # Production build

# Go tests
go test ./internal/...          # Run all Go tests
go test ./internal/utils/...    # Run a specific package's tests
go test -v ./internal/utils/... # Verbose output for debugging
```

The frontend package manager is **bun** (configured in `wails.json`); use `bun` not `npm`.
Go version: **1.24.3**.

## Architecture

This is a **Wails v2** desktop application: a Go backend embedded with a React/TypeScript frontend. The app simulates drone telemetry data and pushes it to backend servers over TCP/UDP.

### Go backend (`/`)

```
app.go                  Wails-bound struct — exposes RPC methods to the frontend
internal/
  config/config.go      Config struct + DefaultConfig(); GlobalConfig singleton
  engine/engine.go      Starts/stops all four modules concurrently; 5s shutdown timeout
  engine/logger.go      LogWriter — ring-buffer that forwards log lines to frontend via Wails event "log"
  modules/              One file per module (analysis, fpv, jamming, detection_udp_server)
  mocks/                MockDataGenerator — simulates drone telemetry with realistic GPS motion
  scenes/scenes.go      JSON scene presets persisted to ~/.virtual-device-ui/scenes/
  protocol/jamming.go   Wire protocol for the jamming module
  utils/                Geo math (CalculateDestination, CalculateBearing, CalculateDistance) + helpers
```

**Data flow:** `Config` → `engine.Start()` → creates a `MockDataGenerator` (shared by all modules) → four goroutines run the modules. Each module pushes simulated data over TCP/UDP to a backend server. The engine also runs an optional ticker to refresh random drone positions.

**Module types** (defined in `modules/modules.go`):
- `analysis` — TCP client; parses and pushes drone identification data (DID/RID/O3+/O4 encrypted packets)
- `detection` — UDP server; receives direction-finding commands from clients, responds with spectrum/direction data
- `fpv` — TCP client; pushes FPV video link data
- `jamming` — TCP client; receives RF strike commands and responds with acknowledgments

**Key dependencies:**
- `github.com/brianvoe/gofakeit/v7` — realistic fake data generation for drone telemetry
- `github.com/sourcegraph/conc` — structured concurrency for goroutine management

### Mock data generation (`internal/mocks/`)

`MockDataGenerator` maintains a pool of drones (predefined + randomly generated). Each drone has fixed attributes (serial, model, freq, detection model) but position/RSSI vary per packet. O3+/O4 drones use encrypted packets; DID/RID drones use standard protocols. Supports loading predefined O3+/O4 packet data from file (`Analysis.O3PlusO4DataFile`).

### Frontend (`frontend/src/`)

Calls Go methods via Wails-generated bindings in `wailsjs/go/main/App` (auto-generated, not committed). The generated types live in `wailsjs/go/models.ts`.

```
App.tsx              Root: polls GetStatus every 1.5s, owns cfg + status state; subscribes to "log" event
components/
  ConfigEditor.tsx   Edits the full Config struct (disabled when engine running)
  ModuleStatus.tsx   Shows engine running state + Start/Stop controls
  LogViewer.tsx      Subscribes to Wails event "log"; uses react-virtuoso for virtual scrolling
  SceneSelector.tsx  Load/save/rename/delete/import/export scene presets via Go RPC
  MapPicker.tsx      Leaflet map for selecting center_point; uses Nominatim geocoder for place search
i18n/                i18next translations — en.ts and zh.ts (toggle via header button)
```

### Frontend-backend communication

- **RPC methods:** StartEngine, StopEngine, GetStatus, GetDefaultConfig, SyncCurrentConfig, Scene CRUD, SearchPlaces, GetLocalIPs
- **Events:** Backend emits `"log"` event with log lines; frontend `LogViewer` subscribes and displays
- **Config sync:** `SyncCurrentConfig` is called on every config change to enable dirty-checking on app close (prompts to save if modified)

### Scene persistence

Scenes are JSON files in `~/.virtual-device-ui/scenes/`. Import/export supported via system file dialogs. Two default scenes (Shanghai, Beijing) are seeded on first launch.

### Environment variables

- `VIRTUAL_DEVICE_UI_GEOCODER_SEARCH_URL` — override Nominatim geocoder endpoint (default: `https://nominatim.openstreetmap.org/search`)

### Key config fields

- `drone_count` + `predefined_drones` — used together; total drones = predefined count + drone_count
- `center_point` (GPS) + `max_distance_from_center_point` — bounds drone simulation area
- `random_drone_refresh_interval` — seconds between drone pool refresh (0 = disabled)
- `min_push_speed` / `max_push_speed` — ms delay range between packets (applied to all modules)
- Per-module: `enabled`, `deviceID`, `hosts[]` (except detection uses single `host`), `port`
- Analysis-specific: `empty_packet_probability` (0-100), `o3_plus_o4_data_file` (optional predefined packet file)
