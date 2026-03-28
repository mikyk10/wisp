<p align="center">
  <img src="docs/logo.svg" width="120" height="120" alt="WiSP logo">
</p>

<h1 align="center">WiSP</h1>

<p align="center">
  DIY battery-powered digital photo frame using Waveshare e-Paper displays and ESP32 microcontrollers.
</p>

<p align="center">
  <a href="https://github.com/mikyk10/wisp/actions/workflows/ci-api.yml"><img src="https://github.com/mikyk10/wisp/actions/workflows/ci-api.yml/badge.svg" alt="CI - API"></a>
  <a href="https://github.com/mikyk10/wisp/actions/workflows/ci-frontend.yml"><img src="https://github.com/mikyk10/wisp/actions/workflows/ci-frontend.yml/badge.svg" alt="CI - Frontend"></a>
  <a href="https://github.com/mikyk10/wisp/actions/workflows/ci-firmware.yml"><img src="https://github.com/mikyk10/wisp/actions/workflows/ci-firmware.yml/badge.svg" alt="CI - Firmware"></a>
</p>

---

> **Experimental.** This project is a work in progress. APIs, configuration formats, and hardware targets may change without notice. Use at your own risk.

## Components

| Directory | Role |
|---|---|
| [`api/`](api/README.md) | Go backend — indexes photos, serves e-Paper binary to ESP32 and JSON API to the frontend |
| [`frontend/`](frontend/README.md) | Vue 3 SPA — browse and manage photo catalogs |
| [`firmware/`](firmware/README.md) | ESP32 firmware (PlatformIO / Arduino C++) |

## Quick start

**Prerequisites:** Docker, Make, curl

```sh
# 1. Generate config files, .env, and sample photos
make dev-setup

# 2. Start the stack and index photos
make up
```

> **Why is scanning needed?**
> WiSP selects photos randomly from a database index, not by scanning the directory
> on every request. `catalog scan` registers each photo's path and metadata into the
> database so it can be included in random selection. The photo file itself is read
> at request time. The directory is **not watched in real time** — re-run `make scan`
> whenever you add or remove photos.

Re-run `make scan` whenever you add or remove photos.

| Service | URL |
|---|---|
| Frontend | http://localhost:8080 |
| API | http://localhost:9002 |

## Configuration

`make dev-setup` creates these files from their `.example` counterparts:

- **`api/config/config.yaml`** — port, log level, database (SQLite by default)
- **`api/config/service.yaml`** — photo catalogs and display definitions

Edit `api/config/service.yaml` to add your display:

```yaml
displays:
  - name: my-frame
    mac_address: a1b2c3d4e5f6   # your ESP32's MAC address
    model: ws7in3e
    ...
```

See the `.example` files for all available options and color reduction settings.

### HTTP catalogs

In addition to local photo directories, WiSP supports **HTTP catalogs** that pull images from external services. Point an HTTP catalog at any endpoint that returns an image — dashboards, AI-generated artwork, weather maps, or any other visual content.

### Accessing from another device on the network

Set `API_BASE_URL` in `.env` to your machine's LAN IP so the browser can reach the API:

```sh
# .env
PHOTO_DIR=/path/to/photos
API_BASE_URL=http://192.168.1.10:9002
```

## Firmware

See [`firmware/`](firmware/) for build and flash instructions.
Configure Wi-Fi credentials and the API server URL via the SoftAP setup on first boot.
The SoftAP network address (`192.168.254.1`) and the SSID/hostname template (`WISP-AP-XXXXXX`) are defined in `firmware/src/config/network.h`.
After connecting to Wi-Fi, the device registers itself as `wisp.local` via mDNS (reachable at `http://wisp.local/` on the same network).
