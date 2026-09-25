#if !defined(ESP8266)
#error This code is intended to run only on the ESP8266 boards ! Please check your Tools->Board setting.
#endif

#define _WEBSOCKETS_LOGLEVEL_ 2

#include <Arduino.h>
#include <ESP8266WiFi.h>
#include <ESP8266WiFiMulti.h>
#include <ArduinoJson.h>
#include <WebSocketsClient_Generic.h>
#include <Hash.h>
#include <ESP8266HTTPClient.h>
#include <WiFiClient.h>

ESP8266WiFiMulti WiFiMulti;
WebSocketsClient webSocket;

#define WS_SERVER "192.168.254.107"  // also the http server where API lives
#define WS_PORT 9000

bool alreadyConnected = false;
bool alreadyFetched = false;

#define SCAN_MESSAGE_TYPE "scan"
#define HTTP_MESSAGE_TYPE "http"

struct LocationLeds {
  int location;
  int green;
  int yellow;
  int red;
};

// Tray location -> LED pins. Only wired locations are listed; add a row per new location.
const LocationLeds LOCATION_LEDS[] = {
  { 1, D4, D5, D3 },
  { 2, D6, D7, D8 },
};
const int LOCATION_LEDS_COUNT = sizeof(LOCATION_LEDS) / sizeof(LOCATION_LEDS[0]);

void setupLeds() {
  for (int i = 0; i < LOCATION_LEDS_COUNT; i++) {
    const LocationLeds& leds = LOCATION_LEDS[i];
    const int pins[] = { leds.green, leds.yellow, leds.red };

    for (int pin : pins) {
      pinMode(pin, OUTPUT);
      digitalWrite(pin, LOW);
    }
  }
}

const LocationLeds* findLocationLeds(int location) {
  for (int i = 0; i < LOCATION_LEDS_COUNT; i++) {
    if (LOCATION_LEDS[i].location == location) {
      return &LOCATION_LEDS[i];
    }
  }

  return nullptr;
}

// Returns the pin to light for a status, or -1 when the status is unknown.
int statusPin(const LocationLeds& leds, const char* status) {
  if (strcmp(status, "good") == 0) return leds.green;
  if (strcmp(status, "near") == 0) return leds.yellow;
  if (strcmp(status, "expire") == 0) return leds.red;

  return -1;
}

// Lights the status LED of the message's location and turns the other two off.
// Messages of another type, bad fields or unwired locations are ignored.
void applyLocationStatus(JsonObjectConst message, const char* expectedType) {
  const char* type = message["type"] | "";
  if (strcmp(type, expectedType) != 0) {
    return;
  }

  if (!message["location"].is<int>()) {
    return;
  }

  int location = message["location"];
  const char* status = message["status"] | "";

  const LocationLeds* leds = findLocationLeds(location);
  if (leds == nullptr) {
    return;
  }

  int pin = statusPin(*leds, status);
  if (pin < 0) {
    return;
  }

  digitalWrite(leds->green, pin == leds->green ? HIGH : LOW);
  digitalWrite(leds->yellow, pin == leds->yellow ? HIGH : LOW);
  digitalWrite(leds->red, pin == leds->red ? HIGH : LOW);

  Serial.printf("[LED] location %d -> %s\n", location, status);
}

void webSocketEvent(const WStype_t& type, uint8_t* payload, const size_t& length) {
  switch (type) {
    case WStype_DISCONNECTED:
      if (alreadyConnected) {
        Serial.println("[WSc] Disconnected!");
        alreadyConnected = false;
      }

      break;

    case WStype_CONNECTED:
      {
        alreadyConnected = true;

        Serial.print("[WSc] Connected to url: ");
        Serial.println((char*)payload);

        // send message to server when Connected
        webSocket.sendTXT("Connected");
      }
      break;

    case WStype_TEXT:
      onWebSocketMessage(payload, length);

      break;

    case WStype_BIN:
      Serial.printf("[WSc] get binary length: %u\n", length);
      hexdump(payload, length);

      // send data to server
      webSocket.sendBIN(payload, length);
      break;

    case WStype_PING:
      // pong will be send automatically
      Serial.printf("[WSc] get ping\n");
      break;

    case WStype_PONG:
      // answer to a ping we send
      Serial.printf("[WSc] get pong\n");
      break;

    case WStype_ERROR:
    case WStype_FRAGMENT_TEXT_START:
    case WStype_FRAGMENT_BIN_START:
    case WStype_FRAGMENT:
    case WStype_FRAGMENT_FIN:
      break;

    default:
      break;
  }
}

void setup() {
  // Serial.begin(921600);
  Serial.begin(115200);

  while (!Serial)
    ;

  delay(200);

  setupLeds();

  Serial.print("\nStart ESP8266_WebSocketClient on ");
  Serial.println(ARDUINO_BOARD);
  Serial.println(WEBSOCKETS_GENERIC_VERSION);

  WiFi.mode(WIFI_STA);
  WiFiMulti.addAP("tea2.4", "84753620Aa!");

  while (WiFiMulti.run() != WL_CONNECTED) {
    Serial.print(".");
    delay(100);
  }

  Serial.println();

  // Client address
  Serial.print("WebSockets Client started @ IP address: ");
  Serial.println(WiFi.localIP());

  // server address, port and URL
  Serial.print("Connecting to WebSockets Server @ ");
  Serial.println(WS_SERVER);

  // server address, port and URL
  webSocket.begin(WS_SERVER, WS_PORT, "/ws");


  // event handler
  webSocket.onEvent(webSocketEvent);

  // try ever 5000 again if connection has failed
  webSocket.setReconnectInterval(5000);

  // start heartbeat (optional)
  // ping server every 15000 ms
  // expect pong from server within 3000 ms
  // consider connection disconnected if pong is not received 2 times
  webSocket.enableHeartbeat(15000, 3000, 2);

  // server address, port and URL
  Serial.print("Connected to WebSockets Server @ IP address: ");
  Serial.println(WS_SERVER);
}

void loop() {
  fetchCurrentStatus();
  webSocket.loop();
}


void onWebSocketMessage(uint8_t* payload, size_t length) {
  JsonDocument doc;
  DeserializationError error = deserializeJson(doc, payload, length);

  if (error) {
    return;
  }

  Serial.print("[WSc] JSON: ");
  serializeJson(doc, Serial);
  Serial.println();

  applyLocationStatus(doc.as<JsonObjectConst>(), SCAN_MESSAGE_TYPE);
}

void applyCurrentStatus(const String& payload) {
  JsonDocument doc;
  DeserializationError error = deserializeJson(doc, payload);

  if (error) {
    Serial.printf("[HTTP] invalid JSON: %s\n", error.c_str());
    return;
  }

  for (JsonObjectConst item : doc.as<JsonArrayConst>()) {
    applyLocationStatus(item, HTTP_MESSAGE_TYPE);
  }
}

void fetchCurrentStatus() {

  if (alreadyFetched) {
    return;
  }

  WiFiClient client;

  HTTPClient http;

  Serial.print("[HTTP] begin...\n");
  if (http.begin(client, "http://" WS_SERVER ":8080/api/medicines/locations")) {  // HTTP


    Serial.print("[HTTP] GET...\n");
    // start connection and send HTTP header
    int httpCode = http.GET();

    // httpCode will be negative on error
    if (httpCode > 0) {
      // HTTP header has been send and Server response header has been handled
      Serial.printf("[HTTP] GET... code: %d\n", httpCode);

      // file found at server
      if (httpCode == HTTP_CODE_OK || httpCode == HTTP_CODE_MOVED_PERMANENTLY) {
        String payload = http.getString();
        Serial.println(payload);
        applyCurrentStatus(payload);
      }
    } else {
      Serial.printf("[HTTP] GET... failed, error: %s\n", http.errorToString(httpCode).c_str());
    }

    http.end();
  } else {
    Serial.println("[HTTP] Unable to connect");
  }

  alreadyFetched = true;
}