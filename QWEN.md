# QWEN.md — virtual-device-ui

## Project Overview

**virtual-device-ui** is a desktop application built with [Wails](https://wails.io), combining a **Go backend** and a **React + TypeScript frontend**. It simulates virtual drone devices and provides a UI for configuring, launching, and monitoring mock drone data streams.

### Core Purpose
The app acts as a **virtual drone simulator** that generates mock data for multiple drone-related modules:
- **Analysis** — sends simulated drone telemetry packets (with configurable empty-packet probability)
- **Detection** — UDP server that simulates drone direction detection data
- **FPV** — simulates first-person-view video feed data
- **Jamming** — simulates jamming/interference module data

Users can configure drone parameters (GPS coordinates, frequency, signal strength, etc.), manage scene presets, pick locations on a map (Leaflet), and monitor module connection status in real time.

### Architecture

```
virtual-device-ui/
├── main.go              # Wails app entry point, bootstraps the desktop window
├── app.go               # App struct — exposes Go methods to the frontend via Wails bridge
├── internal/
│   ├── config/          # Config types (Config, BaseConfig, AnalysisConfig, DetectionConfig, etc.) + validation
│   ├── engine/          # Engine — orchestrates lifecycle of all mock modules
│   ├── modules/         # Feature modules (Analysis, FPV, Detection, Jamming)
│   ├── mocks/           # MockDataGenerator — generates simulated drone data
│   ├── protocol/        # Shared protocol types
│   ├── scenes/          # Scene preset manager (save/load/export/import JSON presets)
│   └── utils/           # Reusable helpers (geo, math) + unit tests
├── frontend/
│   ├── src/
│   │   ├── App.tsx      # Main React component
│   │   ├── components/  # ConfigEditor, ModuleStatus, LogViewer, MapPicker, SceneSelector, etc.
│   │   ├── i18n/        # Internationalization (Chinese + English)
│   │   └── hooks/       # Custom React hooks
│   └── wailsjs/         # Auto-generated Wails bridge (Go → TypeScript bindings)
└── build/               # Desktop app build outputs and packaging assets
```

### Technologies

| Layer | Stack |
|-------|-------|
| Backend | Go 1.24+, Wails v2 |
| Frontend | React 18, TypeScript, Vite 3 |
| UI | Leaflet (maps), i18next (i18n) |
| Package manager | Bun (frontend), Go modules (backend) |
| Desktop packaging | Wails (native WebView) |

---

## Building and Running

### Development
```bash
# Run the full desktop app with live frontend reload
wails dev

# Alternatively, start the frontend in isolation
cd frontend && bun run dev
```

### Production Build
```bash
# Build a redistributable desktop bundle
wails build
```

### Frontend-only Commands
```bash
cd frontend
bun run dev       # Vite dev server
bun run build     # Type-check + bundle for production
bun run preview   # Preview production build
```

### Testing
```bash
# Run all Go tests
go test ./...
```

---

## Key Go Backend APIs (exposed to frontend via Wails)

| Method | Description |
|--------|-------------|
| `StartEngine(cfg)` | Start the virtual device engine with the given config |
| `StopEngine()` | Stop all modules |
| `GetStatus()` | Return engine + module connection status |
| `GetDefaultConfig()` | Return the default configuration |
| `SyncCurrentConfig(cfg)` | Sync the current edited config to the backend |
| `LoadScene(name)` | Load a saved scene preset |
| `SaveScene(name, cfg)` | Save current config as a scene |
| `DeleteScene(name)` | Delete a scene |
| `RenameScene(old, new)` | Rename a scene |
| `ListScenes()` | List all scene names |
| `ImportScene()` | Import a scene from a JSON file |
| `ExportScene(name)` | Export a scene to a JSON file |
| `SearchPlaces(query)` | Geocode search via Nominatim (OpenStreetMap) |
| `SelectFile(path, title)` | Open file dialog |
| `SelectDirectory(path, title)` | Open directory dialog |
| `GetLocalIPs()` | Return all non-loopback IPv4 addresses |

---

## Configuration Model

The main `config.Config` struct includes:

- **Global settings**: `drone_count`, `random_drone_refresh_interval`, `min_push_speed`, `max_push_speed`, `max_direction_change`, `max_distance_from_center_point`, `center_point`
- **Predefined drones**: Array of `PredefinedDrone` with `serial`, `model`, `freq`, `rssi`, `drone_gps`, `pilot_gps`, `type` (DID/RID/AUTO)
- **Module configs**: `analysis`, `detection`, `fpv`, `jamming` — each with `enabled`, `deviceID`, `hosts`/`host`, `port`, and module-specific fields

---

## Development Conventions

### Go
- Format with `gofmt -w`
- Tabs for indentation, `PascalCase` for exported identifiers, `camelCase` for unexported
- Table-driven tests in `*_test.go` files
- Tests in `internal/utils/` and `internal/scenes/`

### Frontend
- Strict TypeScript, functional React components
- `PascalCase` filenames (e.g., `ConfigEditor.tsx`), `camelCase` for props/hooks
- Internationalization via `i18next` — translations in `frontend/src/i18n/{zh,en}.ts`
- Wails bridge calls kept close to the UI boundary

### Commits
- Short, imperative subjects (e.g., `Add scene rename validation`)
- One logical change per commit

---

## Important Notes

- When exported Go APIs change, regenerate the bindings under `frontend/wailsjs` and commit them.
- The app window title is localized: `虚拟设备模拟器` (Chinese).
- Geocoder search uses `nominatim.openstreetmap.org` by default; overrideable via `VIRTUAL_DEVICE_UI_GEOCODER_SEARCH_URL` env var.
- The `beforeClose` hook in `app.go` prompts the user to save unsaved changes as a scene preset before exiting.
