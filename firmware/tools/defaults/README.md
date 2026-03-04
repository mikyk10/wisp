# WiSP Default Image Generator

Generates per-device default display images (`.bin`) for the WiSP firmware fallback.
These images are served from GitHub Pages when the server URL is not configured or the primary server is unreachable.

## How it works

```
src/<device>.html        (static HTML with embedded fonts and SVG)
  → Playwright           (renders in browser → captures PNG)
  → wisp image convert   (PNG → device-specific .bin)
  → ../../docs/defaults/<device>.bin  (served via GitHub Pages)
```

## Supported devices

| File | Resolution | --display |
|---|---|---|
| `epd7in3e` | 600×400 | `ws7in3e` |
| `epd4in0e` | 600×400 | `ws4in0e` |
| `epd13in3e` | 1200×1600 | `ws13in3e` |

## Setup (first time only)

```bash
# Build the wisp CLI
cd ../../../api && make build && cd -

# Set up Playwright
npm install
npx playwright install chromium
```

## Usage

```bash
make                                   # generate all devices
make ../../docs/defaults/epd7in3e.bin  # generate a specific device
```

Commit the generated `.bin` files under `../../docs/defaults/` to publish them to GitHub Pages.

## Notes on HTML design

- Set the viewport size to match the device resolution (see `SIZE_*` in the Makefile)
- Embed custom fonts as base64 via `@font-face` in `<style>` — external URLs are not available under `file://`
- SVG assets referenced from `src/` are loaded as local files and work without inlining
