#ifndef EPAPER_DISPLAY_H
#define EPAPER_DISPLAY_H

#include <Arduino.h>
#include <HTTPClient.h>
#include <esp_sleep.h>
#include <driver/gpio.h>
#include "config/network.h"

class EPaperDisplay {
public:
    static constexpr unsigned long EPD_BUSY_TIMEOUT_MS   = 60000;
    static constexpr unsigned long EPD_STREAM_TIMEOUT_MS = 30000;
    static constexpr uint64_t      EPD_ERROR_SLEEP_US    = 3600ULL * 1000000ULL;

    // Timer deep sleep with the panel's power switch held LOW. Deep sleep
    // releases every ordinary GPIO, so without the hold EPD_PWR_PIN floats
    // for the whole sleep — harmless only if the board has its own pulldown.
    // moduleInit() releases the hold again on the next wake.
    //
    // Every sleep in the firmware — deepSleep() and both sleepOnError
    // variants — ends here, so this is where the duration bounds live and
    // cannot be bypassed by a caller that forgets them.
    [[noreturn]] static void startTimedDeepSleep(uint64_t us) {
        constexpr uint64_t kMinUs = (uint64_t)SLEEP_MIN_SECONDS * 1000000ULL;
        constexpr uint64_t kMaxUs = (uint64_t)SLEEP_MAX_SECONDS * 1000000ULL;
        if (us < kMinUs) us = kMinUs;
        if (us > kMaxUs) us = kMaxUs;
        #ifdef EPD_PWR_PIN
        gpio_hold_en((gpio_num_t)EPD_PWR_PIN);
        gpio_deep_sleep_hold_en();
        #endif
        esp_sleep_enable_timer_wakeup(us);
        esp_deep_sleep_start();
    }

    // For error paths where no display instance exists (factory returned
    // nullptr): nothing was ever powered, so there is no panel to fold.
    [[noreturn]] static void sleepOnErrorNoPanel(const char* reason) {
        Serial.printf("[EPD] %s — entering deep sleep\n", reason);
        startTimedDeepSleep(EPD_ERROR_SLEEP_US);
    }

    // Every caller of this sits past moduleInit(), so the panel is powered —
    // and the MCU's deep sleep does not cut external power. Fold the panel
    // first or it draws current for the entire error sleep, deepening the
    // very supply sag that likely brought us here.
    [[noreturn]] void sleepOnError(const char* reason) {
        static bool inFailsafe = false;
        Serial.printf("[EPD] %s — folding panel, entering deep sleep\n", reason);
        if (!inFailsafe) {
            // A failsafe that fails again must not recurse; skip straight to sleep.
            inFailsafe = true;
            failsafePanelOff();
        }
        startTimedDeepSleep(EPD_ERROR_SLEEP_US);
    }

    virtual void initialize() = 0;
    virtual void sendClearScreenData(unsigned char color) = 0;
    virtual void sendImageData(HTTPClient* client, int length) = 0;
    virtual void sendErrorScreen() = 0;
    virtual void displayImage() = 0;
    virtual void enterSleep() = 0;

    virtual ~EPaperDisplay() {}

protected:
    // Best-effort shutdown for a panel in an unknown state. The default
    // reuses enterSleep(), whose waits are bounded and never call
    // sleepOnError; a driver whose sleep path needs extra recovery (such as
    // a hardware reset first) overrides this.
    virtual void failsafePanelOff() { enterSleep(); }

    // Reads exactly `remaining` bytes from the HTTP stream, handing each
    // chunk to consume(buf, count) — the one home for the read clamp, the
    // failed-read watchdog guard and the stream timeout, shared by every
    // driver. Callers guarantee remaining > 0: fetchImage() rejects unknown
    // lengths, because the raw socket cannot be dechunked here.
    template <typename Consume>
    void streamFromHttp(WiFiClient* stream, int remaining, Consume consume) {
        uint8_t buff[BUF_SIZE];
        unsigned long lastRecv = millis();
        while (remaining > 0) {
            if (stream->available() > 0) {
                int c = stream->read(buff, min(remaining, (int)BUF_SIZE));
                if (c > 0) { // a failed read must not feed the timeout watchdog
                    consume(buff, c);
                    remaining -= c;
                    lastRecv = millis();
                }
            } else if (millis() - lastRecv >= EPD_STREAM_TIMEOUT_MS) {
                sleepOnError("sendImageData stream timeout");
            }
            delay(1);
        }
    }
};

#endif // EPAPER_DISPLAY_H