#ifdef EPD_WAVESHARE_EPD13IN3K

#include <Arduino.h>
#include <HTTPClient.h>
#include <SPI.h>
#include <esp_sleep.h>
#include "EPaperDisplayImpl.h"
#include "EPaperDisplay.h"

#define EPD_WIDTH      960
#define EPD_HEIGHT     680

// Bytes per 1bpp plane: 960 * 680 / 8 = 81,600
#define EPD_PLANE_SIZE (EPD_WIDTH * EPD_HEIGHT / 8)

// Total 2bpp stream size from server: 960 * 680 / 4 = 163,200
#define EPD_STREAM_SIZE (EPD_WIDTH * EPD_HEIGHT / 4)

// LUT for 4-grayscale full refresh (from Waveshare epd13in3k sample, 2023)
static const uint8_t LUT_4GRAY[112] = {
    0x80, 0x48, 0x4A, 0x22, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x0A, 0x48, 0x68, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x88, 0x48, 0x60, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0xA8, 0x48, 0x45, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x07, 0x23, 0x17, 0x02, 0x00,
    0x05, 0x01, 0x05, 0x01, 0x02,
    0x08, 0x02, 0x01, 0x04, 0x04,
    0x00, 0x02, 0x00, 0x02, 0x01,
    0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x01,
    0x22, 0x22, 0x22, 0x22, 0x22,
    0x17, 0x41, 0xA8, 0x32, 0x30,
    0x00, 0x00,
};

// ---------------------------------------------------------------------------
// SPI / GPIO helpers
// ---------------------------------------------------------------------------

void EPD13In3KImpl::spiWrite(unsigned char data) {
    SPI.transfer(data);
}

void EPD13In3KImpl::moduleInit() {
    pinMode(EPD_BUSY_PIN, INPUT);
    pinMode(EPD_RST_PIN,  OUTPUT);
    pinMode(EPD_DC_PIN,   OUTPUT);
    pinMode(EPD_CS_PIN,   OUTPUT);

#ifdef EPD_PWR_PIN
    pinMode(EPD_PWR_PIN, OUTPUT);
    digitalWrite(EPD_PWR_PIN, HIGH);
#endif

    digitalWrite(EPD_CS_PIN, HIGH);

    SPI.begin();
    SPI.beginTransaction(SPISettings(4000000, MSBFIRST, SPI_MODE0));
}

void EPD13In3KImpl::moduleExit() {
#ifdef EPD_PWR_PIN
    digitalWrite(EPD_PWR_PIN, LOW);
#endif
}

void EPD13In3KImpl::sendCommand(unsigned char command) {
    digitalWrite(EPD_DC_PIN, LOW);
    digitalWrite(EPD_CS_PIN, LOW);
    spiWrite(command);
    digitalWrite(EPD_CS_PIN, HIGH);
}

void EPD13In3KImpl::sendData(unsigned char data) {
    digitalWrite(EPD_DC_PIN, HIGH);
    digitalWrite(EPD_CS_PIN, LOW);
    spiWrite(data);
    digitalWrite(EPD_CS_PIN, HIGH);
}

// Wait until BUSY = LOW (idle).
// IMPORTANT: BUSY polarity on epd13in3k is HIGH=busy, LOW=idle.
// This is OPPOSITE to Spectra 6 displays (epd7in3e/epd4in0e/epd13in3e) where LOW=busy.
// NOTE: logging here is verbose; consider removing once stable.
void EPD13In3KImpl::busyLow() {
    Serial.println("[EPD] waiting for idle (busyLow)...");
    unsigned long start = millis();
    while (digitalRead(EPD_BUSY_PIN) == HIGH) {
        if (millis() - start >= EPD_BUSY_TIMEOUT_MS) {
            sleepOnError("busyLow timeout");
        }
        Serial.print(".");
        delay(20);
    }
    delay(20);
    Serial.println(" released.");
}

void EPD13In3KImpl::reset() {
    digitalWrite(EPD_RST_PIN, HIGH); delay(20);
    digitalWrite(EPD_RST_PIN, LOW);  delay(2);
    digitalWrite(EPD_RST_PIN, HIGH); delay(20);
}

// ---------------------------------------------------------------------------
// EPaperDisplay interface
// ---------------------------------------------------------------------------

void EPD13In3KImpl::initialize() {
    moduleInit();
    busyLow();

    sendCommand(0x12);  // SWRESET
    busyLow();

    sendCommand(0x0C);  // Soft start
    sendData(0xAE);
    sendData(0xC7);
    sendData(0xC3);
    sendData(0xC0);
    sendData(0x80);

    sendCommand(0x01);  // Gate setting (680 gates: 0x02A7 = 679)
    sendData(0xA7);
    sendData(0x02);
    sendData(0x00);

    sendCommand(0x11);  // Data entry mode: X inc, Y inc
    sendData(0x03);

    sendCommand(0x44);  // X address range [0, 0x03BF = 959]
    sendData(0x00); sendData(0x00);
    sendData(0xBF); sendData(0x03);

    sendCommand(0x45);  // Y address range [0, 0x02A7 = 679]
    sendData(0x00); sendData(0x00);
    sendData(0xA7); sendData(0x02);

    sendCommand(0x3C);  // Border waveform
    sendData(0x00);

    sendCommand(0x18);  // Use internal temperature sensor
    sendData(0x80);

    sendCommand(0x4E);  // RAM X address counter start
    sendData(0x00); sendData(0x00);

    sendCommand(0x4F);  // RAM Y address counter start
    sendData(0x00); sendData(0x00);

    sendCommand(0x32);  // Write LUT (105 bytes)
    for (int i = 0; i < 105; i++) {
        sendData(LUT_4GRAY[i]);
    }

    sendCommand(0x03);  // Gate driving voltage
    sendData(LUT_4GRAY[105]);

    sendCommand(0x04);  // Source driving voltage
    sendData(LUT_4GRAY[106]);
    sendData(LUT_4GRAY[107]);
    sendData(LUT_4GRAY[108]);

    sendCommand(0x2C);  // VCOM voltage
    sendData(LUT_4GRAY[109]);

    busyLow();
    Serial.println("[display] initialized");
}

// Split a 2bpp-packed byte pair [A, B] (8 pixels total) into one plane0 byte
// (MSBs) and one plane1 byte (LSBs).
//
// Input layout:  A = [P0(7:6), P1(5:4), P2(3:2), P3(1:0)]
//                B = [P4(7:6), P5(5:4), P6(3:2), P7(1:0)]
// Output:
//   plane0 = [P0M, P1M, P2M, P3M, P4M, P5M, P6M, P7M]  (MSBs)
//   plane1 = [P0L, P1L, P2L, P3L, P4L, P5L, P6L, P7L]  (LSBs)
static inline void splitPlanes(uint8_t A, uint8_t B,
                                uint8_t &plane0, uint8_t &plane1) {
    plane0 = ((A & 0x80)     ) | ((A & 0x20) << 1) | ((A & 0x08) << 2) | ((A & 0x02) << 3)
           | ((B & 0x80) >> 4) | ((B & 0x20) >> 3) | ((B & 0x08) >> 2) | ((B & 0x02) >> 1);

    plane1 = ((A & 0x40) << 1) | ((A & 0x10) << 2) | ((A & 0x04) << 3) | ((A & 0x01) << 4)
           | ((B & 0x40) >> 3) | ((B & 0x10) >> 2) | ((B & 0x04) >> 1) | ((B & 0x01)     );
}

// Sends a solid-color screen without buffering.
// color: 0=Black, 1=DarkGray, 2=LightGray, 3=White (2-bit palette index)
void EPD13In3KImpl::sendClearScreenData(unsigned char color) {
    // Pack 4 identical pixels into one 2bpp byte, then derive plane bytes.
    uint8_t packed = (color << 6) | (color << 4) | (color << 2) | color;
    uint8_t plane0, plane1;
    splitPlanes(packed, packed, plane0, plane1);

    sendCommand(0x24);
    for (int i = 0; i < EPD_PLANE_SIZE; i++) {
        sendData(plane0);
    }
    sendCommand(0x26);
    for (int i = 0; i < EPD_PLANE_SIZE; i++) {
        sendData(plane1);
    }
}

// Reads the 2bpp stream from the server, splits into two 1bpp planes in RAM,
// then writes each plane to the display.
//
// Requires ~163 KB of heap (two 81,600-byte plane buffers).
// On ESP32S3 with PSRAM this is fine; without PSRAM it will likely fail.
void EPD13In3KImpl::sendImageData(HTTPClient *client, int length) {
    WiFiClient *stream = client->getStreamPtr();

    uint8_t *plane0 = new uint8_t[EPD_PLANE_SIZE];
    uint8_t *plane1 = new uint8_t[EPD_PLANE_SIZE];
    if (!plane0 || !plane1) {
        delete[] plane0;
        delete[] plane1;
        sleepOnError("sendImageData: plane buffer allocation failed (out of heap)");
    }

    Serial.print("[disp] Buffering 4-gray stream: ");

    int streamRemaining = (length > 0) ? length : EPD_STREAM_SIZE;
    int planeIdx = 0;
    uint8_t rxBuf[BUF_SIZE];
    uint8_t carry = 0;     // leftover byte when read count is odd
    bool hasCarry = false;
    unsigned long lastRecv = millis();

    while (streamRemaining > 0) {
        int avail = stream->available();
        if (avail > 0) {
            int toRead = min(avail, min(streamRemaining, (int)BUF_SIZE));
            int c = stream->read(rxBuf, toRead);
            streamRemaining -= c;
            lastRecv = millis();
            Serial.print(".");

            int i = 0;
            if (hasCarry) {
                // Pair the leftover byte from the previous read with rxBuf[0]
                splitPlanes(carry, rxBuf[0], plane0[planeIdx], plane1[planeIdx]);
                planeIdx++;
                hasCarry = false;
                i = 1;
            }
            for (; i + 1 < c; i += 2) {
                splitPlanes(rxBuf[i], rxBuf[i + 1], plane0[planeIdx], plane1[planeIdx]);
                planeIdx++;
            }
            if (i < c) {
                carry = rxBuf[i];
                hasCarry = true;
            }
        } else if (millis() - lastRecv >= EPD_STREAM_TIMEOUT_MS) {
            delete[] plane0;
            delete[] plane1;
            sleepOnError("sendImageData stream timeout");
        }
        delay(1);
    }
    Serial.println(" done.");

    Serial.print("[disp] Sending plane0 (0x24): ");
    sendCommand(0x24);
    for (int i = 0; i < EPD_PLANE_SIZE; i++) {
        sendData(plane0[i]);
        if (i % (EPD_PLANE_SIZE / 10) == 0) Serial.print(".");
    }
    Serial.println(" done.");

    Serial.print("[disp] Sending plane1 (0x26): ");
    sendCommand(0x26);
    for (int i = 0; i < EPD_PLANE_SIZE; i++) {
        sendData(plane1[i]);
        if (i % (EPD_PLANE_SIZE / 10) == 0) Serial.print(".");
    }
    Serial.println(" done.");

    delete[] plane0;
    delete[] plane1;

    delay(200);
}

// TODO: Replace with a proper RLE-compressed error image (see other EPD drivers).
// Currently sends an all-white screen as a placeholder.
void EPD13In3KImpl::sendErrorScreen() {
    sendCommand(0x24);
    for (int i = 0; i < EPD_PLANE_SIZE; i++) {
        sendData(0xFF);  // White: plane0 MSBs all 1
    }
    sendCommand(0x26);
    for (int i = 0; i < EPD_PLANE_SIZE; i++) {
        sendData(0xFF);  // White: plane1 LSBs all 1
    }
}

void EPD13In3KImpl::turnOnDisplay4Gray() {
    sendCommand(0x22);
    sendData(0xC7);
    sendCommand(0x20);
    busyLow();
}

void EPD13In3KImpl::displayImage() {
    turnOnDisplay4Gray();
}

void EPD13In3KImpl::enterSleep() {
    sendCommand(0x10);  // Deep sleep mode 1 (retains RAM)
    sendData(0x03);
    delay(100);
    busyLow();

    digitalWrite(EPD_RST_PIN, LOW);
    Serial.println("e-Paper in sleep mode");
    moduleExit();

    SPI.endTransaction();
    SPI.end();
}

#endif // EPD_WAVESHARE_EPD13IN3K
