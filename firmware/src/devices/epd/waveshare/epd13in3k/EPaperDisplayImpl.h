#ifndef EPD_WAVESHARE_EPD13IN3K_H
#define EPD_WAVESHARE_EPD13IN3K_H

#include <Arduino.h>
#include <EPaperDisplay.h>
#include <HTTPClient.h>

// 960x680px, 4-grayscale (Black/DarkGray/LightGray/White), 2bpp
// Standard single-IC design with one CS pin (unlike epd13in3e which has two).
//
// BUSY polarity: HIGH = busy, LOW = idle.
// This is INVERTED compared to Spectra 6 displays (epd7in3e, epd4in0e, epd13in3e).
//
// Data format from server (ws13in3EpaperKEncoder):
//   2bpp packed, 4 pixels/byte, MSB-first within each byte.
//   Total stream: 960 * 680 / 4 = 163,200 bytes.
//
// Hardware uses two 1bpp RAM planes:
//   Register 0x24 (BW RAM):   MSB of each 2-bit pixel value  (81,600 bytes)
//   Register 0x26 (GRAY RAM): LSB of each 2-bit pixel value  (81,600 bytes)
//
// Palette:  0=Black (00), 1=DarkGray (01), 2=LightGray (10), 3=White (11)
class EPD13In3KImpl : public EPaperDisplay {
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
    void spiWrite(unsigned char data);
    void sendCommand(unsigned char command);
    void sendData(unsigned char data);
    // Wait until BUSY = LOW (idle). Polarity is HIGH=busy, LOW=idle.
    void busyLow();
    void reset();
    void turnOnDisplay4Gray();
};

#endif // EPD_WAVESHARE_EPD13IN3K_H
