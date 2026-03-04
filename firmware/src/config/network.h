#ifndef CONFIG_H
#define CONFIG_H

#include <WiFi.h>

const IPAddress softap_ip(192, 168, 254, 1);
const IPAddress softap_subnet(255, 255, 255, 0);

const char ssid_template[] = "WISP-AP-******"; // Dynamically replaced by hardware MAC Address
const char hostname_template[] = "WISP-******";  // Dynamically replaced by hardware MAC Address
// Server URL is configured via the WiFi setup page and stored in Preferences (NVS)

// Device-specific fallback image URL (HTTPS, raw.githubusercontent.com)
#if defined(EPD_WAVESHARE_EPD7IN3E)
  #define FALLBACK_IMAGE_URL "https://github.com/mikyk10/wisp/raw/refs/heads/feature/default-bin-image/firmware/docs/defaults/epd7in3e.bin"
#elif defined(EPD_WAVESHARE_EPD4IN0E)
  #define FALLBACK_IMAGE_URL "https://raw.githubusercontent.com/<user>/<repo>/main/docs/defaults/epd4in0e.bin"
#elif defined(EPD_WAVESHARE_EPD13IN3E)
  #define FALLBACK_IMAGE_URL "https://raw.githubusercontent.com/<user>/<repo>/main/docs/defaults/epd13in3e.bin"
#else
  #define FALLBACK_IMAGE_URL ""
#endif

#define FALLBACK_SLEEP_SECONDS 3600

#endif