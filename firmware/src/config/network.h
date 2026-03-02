#ifndef CONFIG_H
#define CONFIG_H

#include <WiFi.h>

const IPAddress softap_ip(192, 168, 254, 1);
const IPAddress softap_subnet(255, 255, 255, 0);

const char ssid_template[] = "WISP-AP-******"; // Dynamically replaced by hardware MAC Address
const char hostname_template[] = "WISP-******";  // Dynamically replaced by hardware MAC Address
// Server URL is configured via the WiFi setup page and stored in Preferences (NVS)

#endif