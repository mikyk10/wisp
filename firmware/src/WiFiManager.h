#ifndef WIFI_MANAGER_H
#define WIFI_MANAGER_H

#ifdef ESP32
#include <Preferences.h>
#else
#error "This code is for ESP32 only!"
#endif

#include <WiFi.h>
#include <WebServer.h>
#include <ESPmDNS.h>
#include "config/network.h"

class WiFiManager
{
private:
    String getMacSuffix();
    String generateSSID();
    String generateHostname();

    WebServer server;

    Preferences preferences;

    void handleRoot();
    void handleSave();
    void handleScan();
    void enableMDNS();

public:
    // channelHint / bssidHint are an optional fast-connect hint (e.g. the values
    // cached in NVS by saveConnHint). When provided, the first attempt skips the
    // channel scan; on failure it automatically falls back to a full-scan connect.
    // Pass channelHint <= 0 / bssidHint == nullptr to always do a full scan.
    bool connectToWiFi(const char *ssid, const char *password, int timeout,
                       int32_t channelHint = 0, const uint8_t *bssidHint = nullptr);
    void startSoftAP();
    void startSoftAPWithWebServer();
    void saveCredentials(const char *ssid, const char *password);
    bool loadCredentials(String &ssid, String &password);
    void saveServerURL(const char *url);
    bool loadServerURL(String &url);

    // Fast-connect hint persisted in NVS (survives power loss). bssid points to 6 bytes.
    // loadConnHint returns false when no valid hint is stored.
    bool loadConnHint(int32_t &channel, uint8_t *bssid);
    void saveConnHint(int32_t channel, const uint8_t *bssid);
};

#endif
