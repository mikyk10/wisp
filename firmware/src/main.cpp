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
    if (!wifiManager.loadCredentials(ssid, password) ||
        !wifiManager.connectToWiFi(ssid.c_str(), password.c_str(), 15000)) {
        Serial.println("[WiFi] Connection failed, showing error and entering SoftAP mode...");

        initEPaper();

        epaper->sendErrorScreen();
        epaper->displayImage();
        epaper->enterSleep();

        // Display error and keep SoftAP active for reconfiguration:
        // - User sees error screen on display (device not broken)
        // - User can access Web UI at http://wisp.local without manual reset
        // - No sleep delay: user gets immediate feedback and can reconfigure
        wifiManager.startSoftAPWithWebServer();
        return;
    }

    initEPaper();

    String serverBaseURL;
    bool hasServerURL = wifiManager.loadServerURL(serverBaseURL);

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
        //TODO: fetch errorが検知できないのを修正する

        // NOTE: sleepSeconds == 0 (server returned X-Sleep-Seconds: 0) falls through to error path.
        // If 0-second sleep becomes a valid server response, change this to >= 0.
        if (sleepSeconds > 0) {
            // 転送が正常終了したとみなす
            epaper->displayImage();
            epaper->enterSleep();
            deepSleep(sleepSeconds);
            return;
        }
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
