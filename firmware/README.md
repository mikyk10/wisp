<img src="../docs/logo.svg" width="100" alt="WiSP">

# WiSP Firmware

Arduino/PlatformIO firmware for ESP32-based e-paper photo frames. Wakes from deep sleep, fetches a binary image from WiSP Server over WiFi, renders it on the e-paper display, then sleeps until the next update.

## Features

- Deep-sleep power management — minimal current draw between updates
- WiFi provisioning via SoftAP web UI — credentials stored in ESP32 NVS, not hardcoded
- BOOT-button config mode — press and release RST then immediately hold BOOT to re-enter provisioning without reflashing
- Supports 7.3″ 7-color and 4.0″ black/white Waveshare displays
- Sleep duration controlled by `X-Sleep-Seconds` response header from server (default 300 s, minimum 180 s)
- Error screen displayed on failed image fetch, followed by 1-hour sleep

## Hardware Requirements

> **No warranty.** Specific hardware combinations are not guaranteed to be compatible or safe. Any damage resulting from the use of this firmware and hardware is your own responsibility.

### Tested MCUs

| MCU | Notes |
|-----|-------|
| Seeed XIAO ESP32-S3 | |
| Seeed XIAO ESP32-C3 | |

### Tested displays

| Display | Resolution | Colors |
|---------|-----------|--------|
| Waveshare EPD7IN3E | 600 × 448 | 7 |
| Waveshare EPD4IN0E | 400 × 300 | 7 |
| Waveshare EPD13IN3E | 1200 × 1600 | 7 |

Combinations other than those reflected in the PlatformIO environments are untested.

### Other parts

| Part | Notes |
|------|-------|
| Li-Ion / LiPo cell | Voltage and capacity depend on your circuit; XIAO includes an onboard charge controller |
| Connecting wires and enclosure | — |

### Pin mapping

**seeed_xiao_esp32s3_epd7in3e** (7.3″ color):

| Signal | GPIO | Seeed label |
|--------|------|-------------|
| EPD_PWR | 2 | D1 |
| EPD_BUSY | 3 | D2 |
| EPD_RST | 4 | D3 |
| EPD_DC | 5 | D4 |
| EPD_CS | 6 | D5 |
| EPD_SCK | 7 | D8 |
| EPD_MOSI | 9 | D10 |

**seeed_xiao_esp32c3_epd4in0e** (4.0″ B/W) uses the same logical mapping on its own GPIO numbers — see `firmware/platformio.ini` for details.

## Getting Started

### Prerequisites

Install [PlatformIO](https://platformio.org/install) (CLI or VS Code extension).

### First-boot WiFi setup

1. Flash the firmware (see Build & Flash below).
2. On first boot the device enters SoftAP provisioning mode automatically. To re-enter provisioning later, press and release RST then immediately hold the BOOT button.
3. Connect to the WiFi network broadcast by the device: `WISP-AP-XXXXXX` (XXXXXX = last 6 hex chars of the ESP32 MAC address). No password.
4. Open `http://192.168.254.1` in a browser.
5. Enter your WiFi SSID, password, and the WiSP Server URL (e.g. `http://192.168.1.100:9002`).
6. Submit — the device saves the credentials to NVS and reboots.

After setup the device operates autonomously.

### Build & Flash

```bash
# Build for 7.3″ color display
pio run -e seeed_xiao_esp32s3_epd7in3e

# Build for 4.0″ B/W display
pio run -e seeed_xiao_esp32c3_epd4in0e

# Flash (replace <env> with the environment name above)
pio run --target upload -e seeed_xiao_esp32s3_epd7in3e

# Build + flash in one step
pio run --target upload -e seeed_xiao_esp32c3_epd4in0e
```

Build artifacts are written to `.pio/build/<env>/firmware.bin`. CI automatically builds both environments and attaches binaries to GitHub Releases on version tags.

## Configuration

WiFi credentials and server URL are configured through the SoftAP web UI and stored in ESP32 NVS (non-volatile storage). They persist across deep-sleep cycles without requiring reflashing.

The SoftAP network address (`192.168.254.1`) and the SSID/hostname template (`WISP-AP-XXXXXX`) are defined in `firmware/src/config/network.h`. Modify these constants if you need a different provisioning network.

Pin mappings and compile-time flags (display model, buffer size, PSRAM) are set as build flags in `firmware/platformio.ini` per environment.

## Usage

### Normal operation

After provisioning, each wake cycle follows this sequence:

1. Connect to the configured WiFi network (15-second timeout).
3. GET `{serverURL}/pf/{MAC}/image/random.bin`.
4. Stream binary image data to the e-paper display.
5. Read the `X-Sleep-Seconds` header from the response (default 300, minimum enforced at 180).
6. Enter deep sleep for that duration.

On any error (WiFi failure, HTTP error, timeout), the firmware displays an error screen and sleeps for 1 hour before retrying.

### Entering config mode

| Action | Result |
|--------|--------|
| Press RST | Normal wake — starts WiFi connection |
| Press and release RST, then immediately hold BOOT | Enters SoftAP provisioning mode |

### Battery notes

TLS is disabled by default to reduce CPU load and battery consumption. Deploy WiSP Server on a private network; do not expose it to the internet.

Removing the red power LED from the Waveshare driver board reduces standby current draw noticeably on long deployments.


## Gallery

<!-- Photos of the finished photo frame will be added here -->

## Contributing

1. Fork the [WiSP monorepo](https://github.com/mikyk10/wisp) and create a branch from `main`.
2. Build both environments (from `firmware/`) before submitting a pull request: `pio run -e seeed_xiao_esp32s3_epd7in3e && pio run -e seeed_xiao_esp32c3_epd4in0e`.
3. CI automatically builds both environments on every pull request.
4. Keep pull requests focused — one concern per PR.

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE.md).
