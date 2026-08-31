#include <Arduino.h>
#include <esp_system.h>
#include <WiFiClientSecure.h>
#include "WiFiManager.h"
#include "config/network.h"

#include "EPaperDisplay.h"
#include "EPaperFactory.h"

WiFiManager wifiManager;
EPaperDisplay* epaper = nullptr;

#define HTTP_TIMEOUT 30000

#define LED 2

// Short tokens, safe to embed in a URL query as-is.
const char* resetReasonName(esp_reset_reason_t r) {
    switch (r) {
        case ESP_RST_POWERON:   return "poweron";
        case ESP_RST_EXT:       return "ext";
        case ESP_RST_SW:        return "sw";
        case ESP_RST_PANIC:     return "panic";
        case ESP_RST_INT_WDT:   return "int_wdt";
        case ESP_RST_TASK_WDT:  return "task_wdt";
        case ESP_RST_WDT:       return "wdt";
        case ESP_RST_DEEPSLEEP: return "deepsleep";
        case ESP_RST_BROWNOUT:  return "brownout";
        case ESP_RST_SDIO:      return "sdio";
        default:                return "unknown";
    }
}

const char* wakeupCauseName(esp_sleep_wakeup_cause_t c) {
    switch (c) {
        case ESP_SLEEP_WAKEUP_TIMER: return "timer";
        case ESP_SLEEP_WAKEUP_EXT0:  return "ext0";
        case ESP_SLEEP_WAKEUP_EXT1:  return "ext1";
        case ESP_SLEEP_WAKEUP_GPIO:  return "gpio";
        case ESP_SLEEP_WAKEUP_UART:  return "uart";
        case ESP_SLEEP_WAKEUP_ULP:   return "ulp";
        case ESP_SLEEP_WAKEUP_UNDEFINED: return "none"; // not a deep-sleep wake
        default:                     return "other";
    }
}

int fetchImage(const char* imageURL, EPaperDisplay* epaper) {
    WiFiClient wifiClient;
    HTTPClient httpClient;

    httpClient.setTimeout(HTTP_TIMEOUT);
    httpClient.setFollowRedirects(HTTPC_STRICT_FOLLOW_REDIRECTS);

    // パースしたいヘッダーは事前に宣言が必要
    const char* xSleepSecondsHeader = "X-Sleep-Seconds";
    const char* requiredHeaders[] = {xSleepSecondsHeader};
    httpClient.collectHeaders(requiredHeaders, 1);

    Serial.printf("[http] Fetching image: %s\n", imageURL);

    bool isHTTPS = strncmp(imageURL, "https://", 8) == 0;
    WiFiClientSecure* secureClient = nullptr;
    bool started;
    if (isHTTPS) {
        secureClient = new WiFiClientSecure();
        secureClient->setInsecure();
        started = httpClient.begin(*secureClient, imageURL);
    } else {
        started = httpClient.begin(wifiClient, imageURL);
    }
    if (!started) {
        Serial.println("[http] Failed to start request");
        delete secureClient;
        return -1;
    }

    int httpCode = httpClient.GET();
    Serial.printf("[http] HTTP status: %d\n", httpCode);
    if (httpCode < 0) {
        // Network-level error (connection refused, DNS failure, timeout, etc.)
        Serial.printf("[http] Network error: %s\n", HTTPClient::errorToString(httpCode).c_str());
        httpClient.end();
        delete secureClient;
        return -1;
    }
    if (httpCode != HTTP_CODE_OK) {
        // HTTP error (4xx/5xx): the server may have sent an error image as the body.
        // Fall through to display it instead of silently falling back to the built-in error screen.
        Serial.printf("[http] HTTP error %d — attempting to render server error image\n", httpCode);
    }

    int contentLength = httpClient.getSize();
    if (contentLength <= 0) {
        Serial.println("[http] No content received");
        httpClient.end();
        delete secureClient;
        return -1;
    }

    int sleepSeconds = 300;
    if (httpClient.hasHeader(xSleepSecondsHeader)) {
       String sls = httpClient.header(xSleepSecondsHeader);
       if (sls.length() > 0) {
           sleepSeconds = sls.toInt();
           Serial.printf("[http] Server requested sleep for %d seconds\n", sleepSeconds);
       }
    }

    epaper->sendImageData(&httpClient, contentLength);

    httpClient.end();
    delete secureClient;

    return sleepSeconds;
}


void deepSleep(int seconds) {
    Serial.printf("[sys] Entering deep sleep for %d seconds...\n", seconds);
    esp_sleep_enable_timer_wakeup(seconds * 1000000ULL);
    esp_deep_sleep_start();
}

void initEPaper() {
    Serial.println("[EPD] Creating display...");
    epaper = EPaperFactory::create();
    if (!epaper) {
        EPaperDisplay::sleepOnError("EPaperFactory::create() returned nullptr — no EPD model defined");
    }
    Serial.println("[EPD] Initializing...");
    epaper->initialize();
    Serial.println("[EPD] Initialized.");
}

void setup() {
    Serial.begin(115200);
    Serial.setDebugOutput(true);
    //delay(5000); // Wait for serial monitor to connect

    Serial.printf("Free heap before new: %d\n", ESP.getFreeHeap());

    // Why this boot happened. A brownout here is the smoking gun for supply
    // sag, and it is unreadable after the next reset — which is why it also
    // rides to the server on the image request below.
    Serial.printf("[sys] reset: %s, wakeup: %s\n",
                  resetReasonName(esp_reset_reason()),
                  wakeupCauseName(esp_sleep_get_wakeup_cause()));

    // Check BOOT button early: press and release RST then immediately hold BOOT to enter config mode
    // Must be checked before the serial delay, as the user holds BOOT right after RST release
    pinMode(BOOT_PIN, INPUT_PULLUP);
    delay(500); // Let pin settle
    if (digitalRead(BOOT_PIN) == LOW) {
        Serial.println("[sys] BOOT held, entering config mode");

        initEPaper();
        epaper->sendErrorScreen();
        epaper->displayImage();
        epaper->enterSleep();
        
        wifiManager.startSoftAPWithWebServer();
        return;
    }


    String ssid, password;

    // No saved credentials → genuinely unconfigured: open SoftAP for setup.
    if (!wifiManager.loadCredentials(ssid, password)) {
        Serial.println("[WiFi] No credentials saved, entering SoftAP setup mode...");

        initEPaper();
        epaper->sendErrorScreen();
        epaper->displayImage();
        epaper->enterSleep();

        wifiManager.startSoftAPWithWebServer();
        return;
    }

    // Credentials exist: try to connect, using a cached channel/BSSID hint to skip
    // the scan on the first attempt (with full-scan fallback inside connectToWiFi).
    int32_t hintChannel = 0;
    uint8_t hintBssid[6];
    bool hasHint = wifiManager.loadConnHint(hintChannel, hintBssid);

    if (!wifiManager.connectToWiFi(ssid.c_str(), password.c_str(), 15000,
                                   hasHint ? hintChannel : 0,
                                   hasHint ? hintBssid : nullptr)) {
        // connectToWiFi already exhausted its in-call retries. Credentials are present,
        // so this is a transient/environmental failure (AP busy, weak signal, server
        // down), not a misconfiguration — show the error screen and go back to a normal
        // sleep. The next wake retries from scratch. We do NOT drop into SoftAP forever
        // (which would need a manual reset to escape); to reconfigure WiFi, hold BOOT at
        // power-on to enter setup mode.
        Serial.printf("[WiFi] Connection failed, showing error and sleeping %ds...\n",
                      FALLBACK_SLEEP_SECONDS);

        wifiManager.shutdownRadio();

        initEPaper();
        epaper->sendErrorScreen();
        epaper->displayImage();
        epaper->enterSleep();

        deepSleep(FALLBACK_SLEEP_SECONDS);
        return;
    }

    // Connected: refresh the fast-connect hint for next wake (no-op if unchanged).
    wifiManager.saveConnHint(WiFi.channel(), WiFi.BSSID());

    initEPaper();

    String serverBaseURL;
    bool hasServerURL = wifiManager.loadServerURL(serverBaseURL);

    int sleepSeconds = -1;

    if (hasServerURL) {
        char imageURL[256];
        uint8_t macAddr[6];
        WiFi.macAddress(macAddr);
        snprintf(imageURL, sizeof(imageURL),
                 "%s/pf/%02x%02x%02x%02x%02x%02x/image/random.bin?rr=%s&wc=%s",
                 serverBaseURL.c_str(),
                 macAddr[0], macAddr[1], macAddr[2], macAddr[3], macAddr[4], macAddr[5],
                 resetReasonName(esp_reset_reason()),
                 wakeupCauseName(esp_sleep_get_wakeup_cause()));
        sleepSeconds = fetchImage(imageURL, epaper);
    }

    // All network work is done — the image bytes are already in the panel's RAM.
    // Cut the radio before the refresh so beacon reception and keep-alive TX bursts
    // don't stack on top of the EPD drive current and deepen the supply sag.
    wifiManager.shutdownRadio();

    // NOTE: sleepSeconds == 0 (server returned X-Sleep-Seconds: 0) falls through to error path.
    // If 0-second sleep becomes a valid server response, change this to >= 0.
    if (sleepSeconds > 0) {
        // Display whatever the server sent (normal image or error image) and sleep.
        epaper->displayImage();
        epaper->enterSleep();
        deepSleep(sleepSeconds);
        return;
    }

    // ここに来ているということはなんか問題があった
    Serial.println("[fallback] Default error page");
    epaper->sendErrorScreen();
    epaper->displayImage();
    epaper->enterSleep();
    deepSleep(86400);
}

void loop() {
    delay(1000);
}
