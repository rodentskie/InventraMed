import type { MedicineStatus } from "../lib/medicine-status"

export type LedColor = MedicineStatus

// Which LEDs in one compartment's cluster are lit. Each LED is independent —
// the hardware may light more than one at a time (e.g. a self-test).
export type LedState = Record<LedColor, boolean>

export interface CompartmentState {
  // 1–12, row-major from the back-left pocket as seen from the front.
  id: number
  leds: LedState
}
