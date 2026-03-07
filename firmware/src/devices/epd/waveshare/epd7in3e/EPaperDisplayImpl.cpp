#ifdef EPD_WAVESHARE_EPD7IN3E

#include <Arduino.h>
#include <HTTPClient.h>
#include <SPI.h>
#include <esp_sleep.h>
#include "EPaperDisplayImpl.h"
#include "EPaperDisplay.h"
#include "../../../../error_icon.h"
#include "../../../../RLEDecoder.h"
#include "../../../../assets/error/epd7in3e.bin.rle.h"

#define EPD_WIDTH       800
#define EPD_HEIGHT      480

static constexpr uint8_t EPD_BLACK = 0x00;
static constexpr uint8_t EPD_WHITE = 0x01;

void EPD7In3EImpl::spiWrite(unsigned char data) {
  SPI.transfer(data);
}

// EPDの電源投入後に1度だけ行う処理
void EPD7In3EImpl::moduleInit()  {
	//gpio
  pinMode(EPD_BUSY_PIN,  INPUT);
  pinMode(EPD_RST_PIN , OUTPUT);
  pinMode(EPD_DC_PIN  , OUTPUT);
  pinMode(EPD_CS_PIN , OUTPUT);

  #ifdef EPD_PWR_PIN
  pinMode(EPD_PWR_PIN  , OUTPUT);
  digitalWrite(EPD_PWR_PIN , HIGH);
  #endif

  digitalWrite(EPD_CS_PIN , HIGH);

	// spi
	SPI.begin();
  SPI.beginTransaction(SPISettings(4000000, MSBFIRST, SPI_MODE0));
}

void EPD7In3EImpl::moduleExit()  {
  #ifdef EPD_PWR_PIN
  digitalWrite(EPD_PWR_PIN , LOW);
  #endif
} 


void EPD7In3EImpl::initialize(){
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

  sendCommand(0x01);//
  sendData(0x3F);

  sendCommand(0x00);  
  sendData(0x5F);
  sendData(0x69);

  sendCommand(0x03);
  sendData(0x00);
  sendData(0x54);
  sendData(0x00);
  sendData(0x44); 

  sendCommand(0x05);
  sendData(0x40);
  sendData(0x1F);
  sendData(0x1F);
  sendData(0x2C);

  sendCommand(0x06);
  sendData(0x6F);
  sendData(0x1F);
  sendData(0x17);
  sendData(0x49);

  sendCommand(0x08);
  sendData(0x6F);
  sendData(0x1F);
  sendData(0x1F);
  sendData(0x22);

  sendCommand(0x30);
  sendData(0x03);
    
  sendCommand(0x50);
  sendData(0x3F);

  sendCommand(0x60);
  sendData(0x02);
  sendData(0x00);

  sendCommand(0x61);
  sendData(0x03);
  sendData(0x20);
  sendData(0x01); 
  sendData(0xE0);

  sendCommand(0x84);
  sendData(0x01);

  sendCommand(0xE3);
  sendData(0x2F);

  sendCommand(0x04);     //PWR on
  busyHigh();          //waiting for the electronic paper IC to release the idle signal

  Serial.println("[display] initialized");
  return;
}

void EPD7In3EImpl::sendClearScreenData(unsigned char color) {
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

void EPD7In3EImpl::sendErrorScreen() {
  sendCommand(0x10);

  RLEDecoder decoder(error_screen_epd7in3e, error_screen_epd7in3e_size);
  int byte;
  while ((byte = decoder.nextByte()) != -1) {
    sendData((uint8_t)byte);
  }
}

void EPD7In3EImpl::sendImageData(HTTPClient *client, int length) {
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

void EPD7In3EImpl::displayImage() {
  sendCommand(0x04);
  busyHigh();
  Serial.println("[displayImage] power on");
  delay(200);

  //Second setting 
  sendCommand(0x06);
  sendData(0x6F);
  sendData(0x1F);
  sendData(0x17);
  sendData(0x49);
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

void EPD7In3EImpl::enterSleep() {
  // POWER_OFF is already sent at the end of displayImage(); skip here to avoid double send.
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

void EPD7In3EImpl::sendCommand(unsigned char command){
  digitalWrite(EPD_DC_PIN, LOW);
  digitalWrite(EPD_CS_PIN, LOW);
  spiWrite(command);
  digitalWrite(EPD_CS_PIN, HIGH);
}

void EPD7In3EImpl::sendData(unsigned char data){
  digitalWrite(EPD_DC_PIN, HIGH);
  digitalWrite(EPD_CS_PIN, LOW);
  spiWrite(data);
  digitalWrite(EPD_CS_PIN, HIGH);
}

void EPD7In3EImpl::busyHigh(){
  //LOW: busy, HIGH: idle
  // NOTE: "Entered busyHigh" and dot logging are verbose; consider removing once stable.
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


void EPD7In3EImpl::reset(){
  digitalWrite(EPD_RST_PIN, HIGH);
  delay(20);
  digitalWrite(EPD_RST_PIN, LOW);
  delay(20);
  digitalWrite(EPD_RST_PIN, HIGH);
  delay(20);
}

#endif