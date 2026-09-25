#ifndef CONFIG_H
#define CONFIG_H

#include <WiFi.h>

const IPAddress softap_ip(192, 168, 254, 1);
const IPAddress softap_subnet(255, 255, 255, 0);

// Normally injected at build time by version_flag.py (git describe); this
// fallback covers builds that bypass PlatformIO's extra_scripts.
#ifndef WISP_FW_VERSION
#define WISP_FW_VERSION "unknown"
#endif

const char ssid_template[] = "WISP-AP-******"; // Dynamically replaced by hardware MAC Address
const char hostname_template[] = "WISP-******";  // Dynamically replaced by hardware MAC Address
// Server URL is configured via the WiFi setup page and stored in Preferences (NVS)

#define FALLBACK_SLEEP_SECONDS 3600

// Bounds for any deep sleep request. The cap stops a corrupted or mistyped
// X-Sleep-Seconds from parking the frame for years (recoverable only via RST);
// it is not an operating policy — a weekly wake schedule (604800) passes
// through untouched. The floor matches the documented minimum and keeps a
// misbehaving server from driving continuous wake/refresh cycles.
#define SLEEP_MIN_SECONDS 180
#define SLEEP_MAX_SECONDS 2592000  // 30 days

#endif