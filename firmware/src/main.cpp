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
    if (httpCode != HTTP_CODE_OK) {
        Serial.printf("[http] Response: %s\n", httpClient.getString().c_str());
        httpClient.end();
        delete secureClient;
        return -1;
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

    epaper->displayImage();

    return sleepSeconds;
}

int fetchImageFallback(EPaperDisplay* epaper) {
    WiFiClientSecure secureClient;
    secureClient.setInsecure();

    HTTPClient httpClient;
    httpClient.setTimeout(HTTP_TIMEOUT);
    httpClient.setFollowRedirects(HTTPC_STRICT_FOLLOW_REDIRECTS);

    Serial.printf("[fallback] Fetching: %s\n", FALLBACK_IMAGE_URL);
    if (!httpClient.begin(secureClient, FALLBACK_IMAGE_URL)) {
        Serial.println("[fallback] Failed to begin request");
        return -1;
    }

    int httpCode = httpClient.GET();
    Serial.printf("[fallback] HTTP status: %d\n", httpCode);
    if (httpCode != HTTP_CODE_OK) {
        httpClient.end();
        return -1;
    }

    int contentLength = httpClient.getSize();
    if (contentLength <= 0) {
        Serial.println("[fallback] No content received");
        httpClient.end();
        return -1;
    }

    epaper->sendImageData(&httpClient, contentLength);
    httpClient.end();
    epaper->displayImage();

    return FALLBACK_SLEEP_SECONDS;
}

void deepSleep(int seconds) {
    Serial.printf("[sys] Entering deep sleep for %d seconds...\n", seconds);
    esp_sleep_enable_timer_wakeup(seconds * 1000000ULL);
    esp_deep_sleep_start();
}

void setup() {
    Serial.begin(115200);
    Serial.setDebugOutput(true);
    //delay(5000); // Wait for serial monitor to connect

    // Check BOOT button early: press RST then immediately hold BOOT to enter config mode
    // Must be checked before the serial delay, as the user holds BOOT right after RST release
    pinMode(BOOT_PIN, INPUT_PULLUP);
    delay(500); // Let pin settle
    if (digitalRead(BOOT_PIN) == LOW) {
        Serial.println("[sys] BOOT held, entering config mode");
        wifiManager.startSoftAPWithWebServer();
        return;
    }

    String ssid, password;
    if (!wifiManager.loadCredentials(ssid, password) ||
        !wifiManager.connectToWiFi(ssid.c_str(), password.c_str(), 15000)) {
        Serial.println("[WiFi] Switching to SoftAP mode with Web UI");
        wifiManager.startSoftAPWithWebServer();
        return;
    }

    String serverBaseURL;
    bool hasServerURL = wifiManager.loadServerURL(serverBaseURL);
    // URL 未設定でも SoftAP には入らず、フォールバックに進む

    Serial.printf("Free heap before new: %d\n", ESP.getFreeHeap());

    Serial.println("[EPD] Creating display...");
    epaper = EPaperFactory::create();
    Serial.println("[EPD] Initializing...");
    epaper->initialize();
    Serial.println("[EPD] Initialized.");

    int sleepSeconds = -1;

    if (hasServerURL) {
        char imageURL[256];
        uint8_t macAddr[6];
        WiFi.macAddress(macAddr);
        snprintf(imageURL, sizeof(imageURL),
                 "%s/pf/%02x%02x%02x%02x%02x%02x/image/random.bin",
                 serverBaseURL.c_str(),
                 macAddr[0], macAddr[1], macAddr[2], macAddr[3], macAddr[4], macAddr[5]);
        sleepSeconds = fetchImage(imageURL, epaper);
    }

    if (sleepSeconds < 180) {
        Serial.println("[fallback] Trying GitHub Pages default...");
        sleepSeconds = fetchImageFallback(epaper);
    }

    if (sleepSeconds >= 180) {
        epaper->enterSleep();
        deepSleep(sleepSeconds);
        return;
    }

    Serial.println("[Epaper] All sources failed, retrying later...");
    epaper->sendErrorScreen();
    epaper->displayImage();
    epaper->enterSleep();
    deepSleep(3600);
}

void loop() {
    delay(1000);
}
