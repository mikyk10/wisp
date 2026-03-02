#ifndef EPD_WAVESHARE_EPD13IN3E_H
#define EPD_WAVESHARE_EPD13IN3E_H

#include <Arduino.h>
#include <EPaperDisplay.h>
#include <HTTPClient.h>

// 1200x1600px, 6-color (Black/White/Yellow/Red/Blue/Green), 4bpp
// Dual-IC design: CS_M controls left half (col 0-599), CS_S controls right half (col 600-1199).
// Uses hardware SPI. DC pin is not used by this driver.
// EPD_CS_S_PIN must be defined in build flags in addition to the standard EPD_* pins.
//
// Expected sendImageData data format from server:
//   [CS_M data: 300 bytes/row * 1600 rows = 480,000 bytes]  <- left panel
//   [CS_S data: 300 bytes/row * 1600 rows = 480,000 bytes]  <- right panel
class EPD13In3EImpl : public EPaperDisplay {
public:
    void initialize() override;
    void sendClearScreenData(unsigned char color) override;
    void sendImageData(HTTPClient *client, int length) override;
    void sendErrorScreen() override;
    void displayImage() override;
    void enterSleep() override;

private:
    void moduleInit();
    void moduleExit();
    void spiSend(uint8_t cmd, const uint8_t *buf, uint32_t len);
    void csAll(uint8_t value);
    void busyHigh();
    void reset();
    void turnOnDisplay();
};

#endif
