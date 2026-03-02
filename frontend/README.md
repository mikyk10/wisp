<img src="docs/logo.svg" width="100" alt="WiSP">

# WiSP Frontend

Vue 3 SPA for browsing and managing photo catalogs served by WiSP Server. Streams photo metadata over NDJSON, renders a virtualized grid, and lets you toggle photo visibility in bulk via the backend REST API.

## Features

- NDJSON streaming photo grid with virtual scrolling — handles thousands of photos without performance degradation
- Timeline sidebar for quick navigation by month and year
- Bulk photo visibility toggle — select multiple photos, then enable or disable them in the backend
- Responsive dark UI — multi-column grid on desktop, two-column on mobile
- Mock mode for development without a running backend
- Security-hardened nginx with enforcing Content Security Policy

## Hardware Requirements

A modern web browser. WiSP Server is required for API mode; mock mode works standalone.

## Getting Started

> **Monorepo users:** to run the full stack (API + frontend together), start from the [repo root](../README.md) instead. The commands below work from within `frontend/` for frontend-only development.

Node 22 is required.

```bash
npm install
```

Start the development server in mock mode (no backend needed):

```bash
npm run dev
```

Open http://localhost:5173 in your browser.

To connect to a running WiSP Server backend, set up `public/env.js` first:

```bash
cp public/env.js.example public/env.js
# Edit public/env.js and set API_BASE_URL to your backend URL, e.g. http://localhost:9002
npm run dev
```

`public/env.js` is gitignored. Do not commit it.

## Configuration

The only runtime configuration value is `window.__env__.API_BASE_URL`, injected at container start in production and loaded from `public/env.js` during local development. No build-time environment variables are used.

| Mode | Condition | Data source |
|------|-----------|-------------|
| Mock | `API_BASE_URL` unset or empty | `/mock-data/photos.ndjson` (bundled) |
| API | `API_BASE_URL` set to a URL | WiSP Server at that URL |

**Production (Docker):** pass `API_BASE_URL` as a container environment variable. `docker-entrypoint.sh` writes the value into `window.__env__` at startup, so a single image works against any backend without rebuilding.

## Usage

### Browsing photos

When the app loads it fetches the catalog list from the backend (API mode) or uses bundled mock data (mock mode). Photos stream into the grid progressively — you can scroll and interact before all photos have loaded. Use the timeline sidebar on the right to jump directly to a specific month or year.

### Selecting and toggling photos

Click any photo to select it. A toolbar appears at the bottom showing the selection count. From the toolbar you can enable or disable the selected photos in the backend, which controls whether the ESP32 frame will display them.

### npm scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Dev server in mock mode (port 5173) |
| `npm run build` | Type-check + production build |
| `npm run preview` | Preview the production build locally |
| `npm run type-check` | vue-tsc type validation |
| `npm run lint` | ESLint check |
| `npm run lint:fix` | ESLint auto-fix |
| `npm run format` | Prettier format |
| `npm run test:unit` | Unit tests in watch mode (Vitest) |
| `npm run test:unit -- --run` | Unit tests, single run |
| `npm run test:e2e` | Playwright E2E tests |

### Docker

For the full stack, use `docker compose up` from the repo root. To build and run the frontend image standalone:

```bash
# Build (run from frontend/)
docker build -t wisp-frontend .

# Run in mock mode
docker run -p 8080:80 wisp-frontend

# Run in API mode
docker run -p 8080:80 -e API_BASE_URL=http://your-backend:9002 wisp-frontend
```

### Testing

**Unit tests (Vitest):** 78 tests across 13 files covering NDJSON streaming, config helpers, Pinia stores (photos, selection, catalogs), the API layer, and components. Coverage thresholds: 70% lines/functions/statements, 60% branches.

```bash
npm run test:unit -- --run
```

**E2E tests (Playwright):** 7 tests covering app boot, photo grid display, photo selection, timeline navigation, and cancel flow.

```bash
npx playwright install chromium   # first time only
npm run test:e2e
```

## Gallery

<!-- Photos of the finished photo frame will be added here -->

## Contributing

1. Fork the [WiSP monorepo](https://github.com/mikyk10/wisp) and create a branch from `main`.
2. Run `npm run lint` and `npm run test:unit -- --run` (from `frontend/`) before submitting a pull request.
3. Keep pull requests focused — one concern per PR.

## License

This project is licensed under the [GNU General Public License v3.0](../LICENSE).
