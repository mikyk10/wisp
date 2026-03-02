#ifndef EPAPER_DISPLAY_H
#define EPAPER_DISPLAY_H

#include <Arduino.h>
#include <HTTPClient.h>
#include <esp_sleep.h>

class EPaperDisplay {
public:
    static constexpr unsigned long EPD_BUSY_TIMEOUT_MS   = 60000;
    static constexpr unsigned long EPD_STREAM_TIMEOUT_MS = 30000;
    static constexpr uint64_t      EPD_ERROR_SLEEP_US    = 3600ULL * 1000000ULL;

    static void sleepOnError(const char* reason) {
        Serial.printf("[EPD] %s — entering deep sleep\n", reason);
        esp_sleep_enable_timer_wakeup(EPD_ERROR_SLEEP_US);
        esp_deep_sleep_start();
    }

    virtual void initialize() = 0;
    virtual void sendClearScreenData(unsigned char color) = 0;
    virtual void sendImageData(HTTPClient* client, int length) = 0;
    virtual void sendErrorScreen() = 0;
    virtual void displayImage() = 0;
    virtual void enterSleep() = 0;

    virtual ~EPaperDisplay() {}
};

#endif // EPAPER_DISPLAY_H