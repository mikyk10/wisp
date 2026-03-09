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
    bool connectToWiFi(const char *ssid, const char *password, int timeout);
    void startSoftAP();
    void startSoftAPWithWebServer();
    void saveCredentials(const char *ssid, const char *password);
    bool loadCredentials(String &ssid, String &password);
    void saveServerURL(const char *url);
    bool loadServerURL(String &url);
};

#endif
