#define LED1 D5
#define LED2 D4
#define LED3 D3

const int LEDS[] = {LED1, LED2, LED3};
const int LED_COUNT = sizeof(LEDS) / sizeof(LEDS[0]);
const int STEP_DELAY = 150;

void setup() {
  for (int i = 0; i < LED_COUNT; i++) {
    pinMode(LEDS[i], OUTPUT);
    digitalWrite(LEDS[i], LOW);
  }
}

void lightOnly(int index) {
  for (int i = 0; i < LED_COUNT; i++) {
    digitalWrite(LEDS[i], i == index ? HIGH : LOW);
  }
  delay(STEP_DELAY);
}

void loop() {
  // Left to right
  for (int i = 0; i < LED_COUNT; i++) {
    lightOnly(i);
  }
  // Right to left, skipping both ends so they don't light twice in a row
  for (int i = LED_COUNT - 2; i > 0; i--) {
    lightOnly(i);
  }
}
