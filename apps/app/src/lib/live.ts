import type { CompartmentState, LedColor, LedState } from "../types/live"
import type { MedicineStatus } from "./medicine-status"

export const COMPARTMENT_COUNT = 12

// Left-to-right order of the LEDs above each pocket, as seen from the front.
export const LED_ORDER: LedColor[] = ["green", "yellow", "red"]

// Brighter than the Chakra status palette so lit LEDs read as light sources.
export const LED_COLORS: Record<LedColor, string> = {
  green: "#2bff5e",
  yellow: "#ffd21f",
  red: "#ff2b2b",
}

const DEMO_STATUSES: (MedicineStatus | null)[] = [
  "green", "green", "yellow", "red",
  "green", "yellow", null, "green",
  "red", "green", "yellow", "green",
]

const RANDOM_STATUSES: (MedicineStatus | null)[] = ["green", "yellow", "red", null]

// Lights exactly the LED matching `status`, or none when there's no status.
export function statusToLeds(status: MedicineStatus | null): LedState {
  return {
    green: status === "green",
    yellow: status === "yellow",
    red: status === "red",
  }
}

function compartmentsFrom(statuses: (MedicineStatus | null)[]): CompartmentState[] {
  return statuses.map((status, i) => ({ id: i + 1, leds: statusToLeds(status) }))
}

export function initialCompartments(): CompartmentState[] {
  return compartmentsFrom(Array.from({ length: COMPARTMENT_COUNT }, () => null))
}

export function demoCompartments(): CompartmentState[] {
  return compartmentsFrom(DEMO_STATUSES)
}

export function randomCompartments(): CompartmentState[] {
  return compartmentsFrom(
    Array.from(
      { length: COMPARTMENT_COUNT },
      () => RANDOM_STATUSES[Math.floor(Math.random() * RANDOM_STATUSES.length)],
    ),
  )
}
