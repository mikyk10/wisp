#ifdef EPD_WAVESHARE_EPD4INE6

#include <Arduino.h>
#include <HTTPClient.h>
#include <SPI.h>
#include <esp_sleep.h>
#include "EPaperDisplayImpl.h"
#include "EPaperDisplay.h"
#include "../../../../error_icon.h"

#define EPD_WIDTH       600
#define EPD_HEIGHT      400

static constexpr uint8_t EPD_BLACK = 0x00;
static constexpr uint8_t EPD_WHITE = 0x01;

void EPD4InE6Impl::spiWrite(unsigned char data) {
  SPI.transfer(data);
}

// EPDの電源投入後に1度だけ行う処理
void EPD4InE6Impl::moduleInit()  {
	//gpio
  pinMode(EPD_BUSY_PIN,  INPUT);
  pinMode(EPD_RST_PIN , OUTPUT);
  pinMode(EPD_DC_PIN  , OUTPUT);
  pinMode(EPD_PWR_PIN  , OUTPUT);
  pinMode(EPD_CS_PIN , OUTPUT);

  digitalWrite(EPD_PWR_PIN , HIGH);
  digitalWrite(EPD_CS_PIN , HIGH);

	// spi
	SPI.begin();
  SPI.beginTransaction(SPISettings(4000000, MSBFIRST, SPI_MODE0));
}

void EPD4InE6Impl::moduleExit()  {
  digitalWrite(EPD_PWR_PIN , LOW);
} 


void EPD4InE6Impl::initialize(){
  moduleInit();

  reset();
  busyHigh();
  delay(30);

  sendCommand(0xAA);    // CMDH
  sendData(0x49);
  sendData(0x55);
  sendData(0x20);
  sendData(0x08);
  sendData(0x09);
  sendData(0x18);

  sendCommand(0x01);
  sendData(0x3F);

  sendCommand(0x00);
  sendData(0x5F);
  sendData(0x69);

  sendCommand(0x05);
  sendData(0x40);
  sendData(0x1F);
  sendData(0x1F);
  sendData(0x2C);

  sendCommand(0x08);
  sendData(0x6F);
  sendData(0x1F);
  sendData(0x1F);
  sendData(0x22);

  sendCommand(0x06);
  sendData(0x6F);
  sendData(0x1F);
  sendData(0x17);
  sendData(0x17);

  sendCommand(0x03);
  sendData(0x00);
  sendData(0x54);
  sendData(0x00);
  sendData(0x44);

  sendCommand(0x60);
  sendData(0x02);
  sendData(0x00);

  sendCommand(0x30);
  sendData(0x08);

  sendCommand(0x50);
  sendData(0x3F);

  sendCommand(0x61);
  sendData(0x01);
  sendData(0x90);
  sendData(0x02);
  sendData(0x58);

  sendCommand(0xE3);
  sendData(0x2F);

  sendCommand(0x84);
  sendData(0x01);
  busyHigh();

  Serial.println("[display] initialized");
  return;
}

void EPD4InE6Impl::sendErrorScreen() {
  static auto iconColor = [](int row, int col, int ix, int iy) -> uint8_t {
    int ir = row - iy, ic = col - ix;
    if (ir < 0 || ir >= ERROR_ICON_HEIGHT || ic < 0 || ic >= ERROR_ICON_WIDTH)
      return EPD_WHITE;
    uint8_t b = pgm_read_byte(&error_icon[ir * ((ERROR_ICON_WIDTH + 7) / 8) + ic / 8]);
    return ((b >> (7 - (ic % 8))) & 1) ? EPD_WHITE : EPD_BLACK;
  };
  const int ix = (EPD_WIDTH  - ERROR_ICON_WIDTH)  / 2;
  const int iy = (EPD_HEIGHT - ERROR_ICON_HEIGHT) / 2;
  sendCommand(0x10);
  for (int row = 0; row < EPD_HEIGHT; row++) {
    for (int col = 0; col < EPD_WIDTH; col += 2) {
      sendData((iconColor(row, col, ix, iy) << 4) | iconColor(row, col + 1, ix, iy));
    }
  }
}

void EPD4InE6Impl::sendClearScreenData(unsigned char color) {
  sendCommand(0x10);
  for (int j = 0; j < EPD_HEIGHT; j++)
  {
      for (int i = 0; i < EPD_WIDTH / 2; i++)
      {
          sendData((color << 4) | color);
      }
      Serial.print(".");
  }
  Serial.println("done");
}

void EPD4InE6Impl::sendImageData(HTTPClient *client, int length) {
    WiFiClient *wifiStream;
    wifiStream = client->getStreamPtr();

    sendCommand(0x10);

    Serial.printf("[disp] Transferring data: ");
        
    uint8_t buff[BUF_SIZE];

    unsigned long lastRecv = millis();
    while(length > 0 || length == -1) {
      size_t size = wifiStream->available();

      if(size) {
        int c = wifiStream->read(buff, BUF_SIZE);
        Serial.printf(".");

        uint8_t *p = buff;
        for (int i = 0; i < c; i++) {
          sendData(*p);
          p++;
        }

        if(length > 0) {
            length -= c;
        }
        lastRecv = millis();
      } else if (millis() - lastRecv >= EPD_STREAM_TIMEOUT_MS) {
        sleepOnError("sendImageData stream timeout");
      }
      delay(1);
    }
    Serial.println("done.");

    delay(200);
}

void EPD4InE6Impl::displayImage() {
  sendCommand(0x04);
  busyHigh();
  Serial.println("[displayImage] power on");
  delay(200);

  //Second setting 
  sendCommand(0x06);
  sendData(0x6F);
  sendData(0x1F);
  sendData(0x17);
  sendData(0x27);
  Serial.println("[displayImage] second setting");
  delay(200);

  sendCommand(0x12); // DISPLAY_REFRESH
  sendData(0x00);
  busyHigh();
  Serial.println("[displayImage] display refresh");

  sendCommand(0x02); // POWER_OFF
  sendData(0X00);
  busyHigh();
  Serial.println("[displayImage] power off");
  delay(200);
}


void EPD4InE6Impl::enterSleep() {
  delay(100);
  sendCommand(0x07);
  sendData(0xA5);
  delay(100);
  digitalWrite(EPD_RST_PIN, 0); // Reset
  Serial.println("e-Paper in sleep mode");
  moduleExit();

  SPI.endTransaction();
  SPI.end();
}

void EPD4InE6Impl::sendCommand(unsigned char command){
  digitalWrite(EPD_DC_PIN, LOW);
  digitalWrite(EPD_CS_PIN, LOW);
  spiWrite(command);
  digitalWrite(EPD_CS_PIN, HIGH);
}

void EPD4InE6Impl::sendData(unsigned char data){
  digitalWrite(EPD_DC_PIN, HIGH);
  digitalWrite(EPD_CS_PIN, LOW);
  spiWrite(data);
  digitalWrite(EPD_CS_PIN, HIGH);
}

void EPD4InE6Impl::busyHigh(){
  //LOW: busy, HIGH: idle
  Serial.println("Entered busyHigh");
  unsigned long start = millis();
  while (!(digitalRead(EPD_BUSY_PIN))){
    if (millis() - start >= EPD_BUSY_TIMEOUT_MS) {
      sleepOnError("busyHigh timeout");
    }
    Serial.print(".");
    delay(100);
  }
  delay(200);
  Serial.println("busy released.");
}


void EPD4InE6Impl::reset(){
  digitalWrite(EPD_RST_PIN, HIGH);
  delay(20);
  digitalWrite(EPD_RST_PIN, LOW);
  delay(2);
  digitalWrite(EPD_RST_PIN, HIGH);
  delay(20);
}

#endif