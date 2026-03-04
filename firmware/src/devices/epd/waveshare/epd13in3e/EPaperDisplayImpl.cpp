#ifdef EPD_WAVESHARE_EPD13IN3E

#include <Arduino.h>
#include <HTTPClient.h>
#include <SPI.h>
#include <esp_sleep.h>
#include "EPaperDisplayImpl.h"
#include "EPaperDisplay.h"
#include "../../../../error_icon.h"
#include "../../../../RLEDecoder.h"
#include "../../../../assets/error/epd13in3e.bin.rle.h"

// Each IC handles 600 columns x 1600 rows, at 4bpp (2 pixels per byte) = 300 bytes/row
#define EPD_WIDTH              1200
#define EPD_HEIGHT             1600
#define EPD_HALF_BYTES_PER_ROW 300  // (EPD_WIDTH / 2) / 2

static constexpr uint8_t EPD_BLACK = 0x00;
static constexpr uint8_t EPD_WHITE = 0x01;

// ---- SPI / GPIO helpers -----------------------------------------------

// Both CS_M (EPD_CS_PIN) and CS_S (EPD_CS_S_PIN) to the same level
void EPD13In3EImpl::csAll(uint8_t value) {
    digitalWrite(EPD_CS_PIN,   value);
    digitalWrite(EPD_CS_S_PIN, value);
}

// Send command byte then data bytes. Caller must manage CS before/after.
void EPD13In3EImpl::spiSend(uint8_t cmd, const uint8_t *buf, uint32_t len) {
    SPI.transfer(cmd);
    for (uint32_t i = 0; i < len; i++) {
        SPI.transfer(buf[i]);
    }
}

// ---- Hardware lifecycle -----------------------------------------------

void EPD13In3EImpl::moduleInit() {
    pinMode(EPD_BUSY_PIN, INPUT);
    pinMode(EPD_RST_PIN,  OUTPUT);
    pinMode(EPD_CS_PIN,   OUTPUT);    // CS_M
    pinMode(EPD_CS_S_PIN, OUTPUT);    // CS_S

    #ifdef EPD_PWR_PIN
    pinMode(EPD_PWR_PIN, OUTPUT);
    digitalWrite(EPD_PWR_PIN, HIGH);
    #endif

    csAll(HIGH);

    SPI.begin();
    SPI.beginTransaction(SPISettings(4000000, MSBFIRST, SPI_MODE0));
}

void EPD13In3EImpl::moduleExit() {
    #ifdef EPD_PWR_PIN
    digitalWrite(EPD_PWR_PIN, LOW);
    #endif
}

// ---- Low-level display protocol ---------------------------------------

// 13in3e requires a double-toggle reset sequence (5 phases)
void EPD13In3EImpl::reset() {
    digitalWrite(EPD_RST_PIN, HIGH); delay(30);
    digitalWrite(EPD_RST_PIN, LOW);  delay(30);
    digitalWrite(EPD_RST_PIN, HIGH); delay(30);
    digitalWrite(EPD_RST_PIN, LOW);  delay(30);
    digitalWrite(EPD_RST_PIN, HIGH); delay(30);
}

void EPD13In3EImpl::busyHigh() {
    Serial.println("Entered busyHigh");
    unsigned long start = millis();
    while (!digitalRead(EPD_BUSY_PIN)) {
        if (millis() - start >= EPD_BUSY_TIMEOUT_MS) {
            sleepOnError("busyHigh timeout");
        }
        Serial.print(".");
        delay(10);
    }
    delay(20);
    Serial.println("busy released.");
}

// PON -> busyHigh -> DRF -> busyHigh -> POF
void EPD13In3EImpl::turnOnDisplay() {
    static const uint8_t drf_v[] = {0x00};
    static const uint8_t pof_v[] = {0x00};

    csAll(LOW);
    SPI.transfer(0x04);  // POWER_ON
    csAll(HIGH);
    busyHigh();

    delay(50);

    csAll(LOW);
    spiSend(0x12, drf_v, sizeof(drf_v));  // DISPLAY_REFRESH
    csAll(HIGH);
    busyHigh();
    Serial.println("[displayImage] display refresh");

    csAll(LOW);
    spiSend(0x02, pof_v, sizeof(pof_v));  // POWER_OFF
    csAll(HIGH);
    Serial.println("[displayImage] power off");
}

// ---- EPaperDisplay interface ------------------------------------------

void EPD13In3EImpl::initialize() {
    moduleInit();
    reset();

    // Command data tables (values from Waveshare ESP32 sample)
    static const uint8_t an_tm_v[]          = {0xC0, 0x1C, 0x1C, 0xCC, 0xCC, 0xCC, 0x15, 0x15, 0x55};
    static const uint8_t cmd66_v[]          = {0x49, 0x55, 0x13, 0x5D, 0x05, 0x10};
    static const uint8_t psr_v[]            = {0xDF, 0x69};
    static const uint8_t cdi_v[]            = {0xF7};
    static const uint8_t tcon_v[]           = {0x03, 0x03};
    static const uint8_t agid_v[]           = {0x10};
    static const uint8_t pws_v[]            = {0x22};
    static const uint8_t ccset_v[]          = {0x01};
    static const uint8_t tres_v[]           = {0x04, 0xB0, 0x03, 0x20};  // 1200x1600
    static const uint8_t pwr_v[]            = {0x0F, 0x00, 0x28, 0x2C, 0x28, 0x38};
    static const uint8_t en_buf_v[]         = {0x07};
    static const uint8_t btst_p_v[]         = {0xE8, 0x28};
    static const uint8_t boost_vddp_en_v[]  = {0x01};
    static const uint8_t btst_n_v[]         = {0xE8, 0x28};
    static const uint8_t buck_boost_vddn_v[]= {0x01};
    static const uint8_t tft_vcom_power_v[] = {0x02};

    // AN_TM: CS_M only (panel-specific timing)
    digitalWrite(EPD_CS_PIN, LOW);
    spiSend(0x74, an_tm_v, sizeof(an_tm_v));
    csAll(HIGH);

    // Shared configuration: both ICs
    csAll(LOW); spiSend(0xF0, cmd66_v, sizeof(cmd66_v)); csAll(HIGH);
    csAll(LOW); spiSend(0x00, psr_v,   sizeof(psr_v));   csAll(HIGH);
    csAll(LOW); spiSend(0x50, cdi_v,   sizeof(cdi_v));   csAll(HIGH);
    csAll(LOW); spiSend(0x60, tcon_v,  sizeof(tcon_v));  csAll(HIGH);
    csAll(LOW); spiSend(0x86, agid_v,  sizeof(agid_v));  csAll(HIGH);
    csAll(LOW); spiSend(0xE3, pws_v,   sizeof(pws_v));   csAll(HIGH);
    csAll(LOW); spiSend(0xE0, ccset_v, sizeof(ccset_v)); csAll(HIGH);
    csAll(LOW); spiSend(0x61, tres_v,  sizeof(tres_v));  csAll(HIGH);

    // Power and boost: CS_M only (master IC drives shared power rails)
    digitalWrite(EPD_CS_PIN, LOW); spiSend(0x01, pwr_v,             sizeof(pwr_v));             csAll(HIGH);
    digitalWrite(EPD_CS_PIN, LOW); spiSend(0xB6, en_buf_v,          sizeof(en_buf_v));           csAll(HIGH);
    digitalWrite(EPD_CS_PIN, LOW); spiSend(0x06, btst_p_v,          sizeof(btst_p_v));           csAll(HIGH);
    digitalWrite(EPD_CS_PIN, LOW); spiSend(0xB7, boost_vddp_en_v,   sizeof(boost_vddp_en_v));    csAll(HIGH);
    digitalWrite(EPD_CS_PIN, LOW); spiSend(0x05, btst_n_v,          sizeof(btst_n_v));           csAll(HIGH);
    digitalWrite(EPD_CS_PIN, LOW); spiSend(0xB0, buck_boost_vddn_v, sizeof(buck_boost_vddn_v));  csAll(HIGH);
    digitalWrite(EPD_CS_PIN, LOW); spiSend(0xB1, tft_vcom_power_v,  sizeof(tft_vcom_power_v));   csAll(HIGH);

    Serial.println("[display] initialized");
}

void EPD13In3EImpl::sendErrorScreen() {
    RLEDecoder decoder(error_screen_epd13in3e, error_screen_epd13in3e_size);

    // CS_M: left panel
    digitalWrite(EPD_CS_PIN, LOW);
    SPI.transfer(0x10);  // DTM
    for (int row = 0; row < EPD_HEIGHT; row++) {
        for (int col = 0; col < EPD_WIDTH / 2; col += 2) {
            uint8_t b1 = (uint8_t)decoder.nextByte();
            uint8_t b2 = (uint8_t)decoder.nextByte();
            SPI.transfer((b1 << 4) | b2);
        }
        delay(1);
    }
    csAll(HIGH);

    // CS_S: right panel
    decoder.reset();
    // Skip to the second half of the data
    for (int i = 0; i < EPD_HEIGHT * EPD_WIDTH / 4; i++) {
        decoder.nextByte();
    }

    digitalWrite(EPD_CS_S_PIN, LOW);
    SPI.transfer(0x10);  // DTM
    for (int row = 0; row < EPD_HEIGHT; row++) {
        for (int col = EPD_WIDTH / 2; col < EPD_WIDTH; col += 2) {
            uint8_t b1 = (uint8_t)decoder.nextByte();
            uint8_t b2 = (uint8_t)decoder.nextByte();
            SPI.transfer((b1 << 4) | b2);
        }
        delay(1);
    }
    csAll(HIGH);
}

void EPD13In3EImpl::sendClearScreenData(unsigned char color) {
    uint8_t pixel = (color << 4) | color;

    // CS_M: left half
    digitalWrite(EPD_CS_PIN, LOW);
    SPI.transfer(0x10);  // DTM
    for (int row = 0; row < EPD_HEIGHT; row++) {
        for (int col = 0; col < EPD_HALF_BYTES_PER_ROW; col++) {
            SPI.transfer(pixel);
        }
        delay(1);
    }
    csAll(HIGH);

    // CS_S: right half
    digitalWrite(EPD_CS_S_PIN, LOW);
    SPI.transfer(0x10);  // DTM
    for (int row = 0; row < EPD_HEIGHT; row++) {
        for (int col = 0; col < EPD_HALF_BYTES_PER_ROW; col++) {
            SPI.transfer(pixel);
        }
        delay(1);
    }
    csAll(HIGH);
}

void EPD13In3EImpl::sendImageData(HTTPClient *client, int length) {
    WiFiClient *stream = client->getStreamPtr();
    uint8_t buff[BUF_SIZE];

    // When Content-Length is unknown (-1), fall back to exact expected size per panel
    int halfLen = (length > 0) ? (length / 2) : (EPD_HALF_BYTES_PER_ROW * EPD_HEIGHT);

    // First half → CS_M (left panel)
    Serial.print("[disp] Transferring CS_M: ");
    digitalWrite(EPD_CS_PIN, LOW);
    SPI.transfer(0x10);  // DTM
    int remaining = halfLen;
    unsigned long lastRecv = millis();
    while (remaining > 0) {
        int avail = stream->available();
        if (avail > 0) {
            int c = stream->read(buff, min(remaining, (int)BUF_SIZE));
            for (int i = 0; i < c; i++) {
                SPI.transfer(buff[i]);
            }
            remaining -= c;
            Serial.print(".");
            lastRecv = millis();
        } else if (millis() - lastRecv >= EPD_STREAM_TIMEOUT_MS) {
            sleepOnError("sendImageData CS_M stream timeout");
        }
        delay(1);
    }
    csAll(HIGH);
    Serial.println("done.");

    // Second half → CS_S (right panel)
    Serial.print("[disp] Transferring CS_S: ");
    digitalWrite(EPD_CS_S_PIN, LOW);
    SPI.transfer(0x10);  // DTM
    remaining = (length > 0) ? (length - halfLen) : halfLen;
    lastRecv = millis();
    while (remaining > 0) {
        int avail = stream->available();
        if (avail > 0) {
            int c = stream->read(buff, min(remaining, (int)BUF_SIZE));
            for (int i = 0; i < c; i++) {
                SPI.transfer(buff[i]);
            }
            remaining -= c;
            Serial.print(".");
            lastRecv = millis();
        } else if (millis() - lastRecv >= EPD_STREAM_TIMEOUT_MS) {
            sleepOnError("sendImageData CS_S stream timeout");
        }
        delay(1);
    }
    csAll(HIGH);
    Serial.println("done.");

    delay(200);
}

void EPD13In3EImpl::displayImage() {
    turnOnDisplay();
}

void EPD13In3EImpl::enterSleep() {
    csAll(LOW);
    SPI.transfer(0x07);  // DEEP_SLEEP
    SPI.transfer(0xA5);
    csAll(HIGH);

    delay(100);
    digitalWrite(EPD_RST_PIN, LOW);
    Serial.println("e-Paper in sleep mode");
    moduleExit();

    SPI.endTransaction();
    SPI.end();
}

#endif
