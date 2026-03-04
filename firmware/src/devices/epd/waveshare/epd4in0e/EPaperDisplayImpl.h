#ifndef EPD_WAVESHARE_EPD4IN0E_H
#define EPD_WAVESHARE_EPD4IN0E_H

#include <Arduino.h>
#include <EPaperDisplay.h>
#include <HTTPClient.h>

class EPD4InE6Impl : public EPaperDisplay {
public:
    void initialize() override;
    void sendClearScreenData(unsigned char color) override;
    void sendImageData(HTTPClient *stream, int length) override;
    void sendErrorScreen() override;
    void displayImage() override;
    void enterSleep() override;

private:
    void moduleInit();
    void spiWrite(unsigned char data);
    void moduleExit();

    void sendCommand(unsigned char command);
    void sendData(unsigned char data);
    void busyHigh();
    void reset();
};

#endif