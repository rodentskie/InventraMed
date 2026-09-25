#if !defined(ESP8266)
#error This code is intended to run only on the ESP8266 boards ! Please check your Tools->Board setting.
#endif

#define _WEBSOCKETS_LOGLEVEL_ 2

#include <ESP8266WiFi.h>
#include <ESP8266WiFiMulti.h>
#include <ArduinoJson.h>
#include <WebSocketsClient_Generic.h>
#include <Hash.h>

ESP8266WiFiMulti WiFiMulti;
WebSocketsClient webSocket;

#define WS_SERVER "192.168.254.107"
#define WS_PORT 9000

bool alreadyConnected = false;

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

  Serial.print("\nStart ESP8266_WebSocketClient on ");
  Serial.println(ARDUINO_BOARD);
  Serial.println(WEBSOCKETS_GENERIC_VERSION);

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
}
