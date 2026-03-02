#include <Arduino.h>
#include <esp_system.h>
#include "WiFiManager.h"

#include "EPaperDisplay.h"
#include "EPaperFactory.h"

WiFiManager wifiManager;
EPaperDisplay* epaper = nullptr;

// ダブルリセット検出: RTC メモリは Deep Sleep / RESET ボタンを跨いで保持される
RTC_DATA_ATTR bool resetPending = false;

#define HTTP_TIMEOUT 30000

#define LED 2

int fetchImage(const char* imageURL, EPaperDisplay* epaper) {
    static WiFiClient wifiClient;
    static HTTPClient httpClient;

    httpClient.setTimeout(HTTP_TIMEOUT);
    httpClient.setFollowRedirects(HTTPC_STRICT_FOLLOW_REDIRECTS);

    // パースしたいヘッダーは事前に宣言が必要
    const char* xSleepSecondsHeader = "X-Sleep-Seconds";
    const char* requiredHeaders[] = {xSleepSecondsHeader};
    httpClient.collectHeaders(requiredHeaders, 1);

    Serial.printf("[http] Fetching image: %s\n", imageURL);
    if (!httpClient.begin(wifiClient, imageURL)) {
        Serial.println("[http] Failed to start request");
        return -1;
    }

    int httpCode = httpClient.GET();
    Serial.printf("[http] HTTP status: %d\n", httpCode);
    if (httpCode != HTTP_CODE_OK) {
        Serial.printf("[http] Response: %s\n", httpClient.getString().c_str());
        httpClient.end();
        return -1;
    }

    int contentLength = httpClient.getSize();
    if (contentLength == 0) {
        Serial.println("[http] No content received");
        httpClient.end();
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

    epaper->displayImage();
    
    return sleepSeconds;
}

void deepSleep(int seconds) {
    Serial.printf("[sys] Entering deep sleep for %d seconds...\n", seconds);
    esp_sleep_enable_timer_wakeup(seconds * 1000000ULL);
    esp_deep_sleep_start();
}

void setup() {
    Serial.begin(115200);
    Serial.setDebugOutput(true);

    Serial.printf("Free heap before new: %d\n", ESP.getFreeHeap());

    // ダブルリセット検出: Deep Sleep 復帰時はスキップ
    esp_reset_reason_t resetReason = esp_reset_reason();
    if (resetReason == ESP_RST_DEEPSLEEP) {
        resetPending = false;
    } else if (resetReason == ESP_RST_EXT) {
        if (resetPending) {
            // 2回目のリセット → SoftAP モードへ
            resetPending = false;
            Serial.println("[sys] Double reset detected, entering config mode");
            wifiManager.startSoftAPWithWebServer();
            return;
        } else {
            // 1回目のリセット: 3秒以内に再度 RESET されたら設定モードへ
            resetPending = true;
            Serial.println("[sys] Press RESET again within 3s to enter config mode");
            delay(3000);
            resetPending = false;
        }
    } else {
        resetPending = false;
    }

    String ssid, password;
    if (!wifiManager.loadCredentials(ssid, password) ||
        !wifiManager.connectToWiFi(ssid.c_str(), password.c_str(), 15000)) {
        Serial.println("[WiFi] Switching to SoftAP mode with Web UI");
        wifiManager.startSoftAPWithWebServer();
        return;
    }

    String serverBaseURL;
    if (!wifiManager.loadServerURL(serverBaseURL)) {
        Serial.println("[config] No server URL saved, entering config mode");
        wifiManager.startSoftAPWithWebServer();
        return;
    }

    epaper = EPaperFactory::create();
    epaper->initialize();

    char imageURL[256];
    uint8_t macAddr[6];
    WiFi.macAddress(macAddr);
    snprintf(imageURL, sizeof(imageURL),
             "%s/pf/%02x%02x%02x%02x%02x%02x/image/random.bin",
             serverBaseURL.c_str(),
             macAddr[0], macAddr[1], macAddr[2], macAddr[3], macAddr[4], macAddr[5]);

    int sleepSeconds = fetchImage(imageURL, epaper);
    if (sleepSeconds >= 180) { // 更新間隔は最短で3分が推奨されているため
        epaper->enterSleep();
        deepSleep(sleepSeconds);
        return;
    }

    Serial.println("[Epaper] something went wrong, retrying later...");
    epaper->sendErrorScreen();
    epaper->displayImage();
    epaper->enterSleep();
    deepSleep(3600);
}

void loop() {
    delay(1000);
}
