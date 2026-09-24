import type {
  CompartmentState,
  LedColor,
  LedState,
  LocationMessage,
  ScanMessage,
  ScanStatus,
} from "../types/live"
import type { Medicine } from "../types/medicine"
import { getMedicineStatus, type MedicineStatus } from "./medicine-status"

export const COMPARTMENT_COUNT = 12

// The locations a medicine can be placed in: one per tray compartment.
export const LOCATIONS: number[] = Array.from({ length: COMPARTMENT_COUNT }, (_, i) => i + 1)

export const SCAN_MESSAGE_TYPE = "scan"

// Type of each item of GET /medicines/locations.
export const LOCATION_MESSAGE_TYPE = "http"

export const SCAN_STATUS: Record<MedicineStatus, ScanStatus> = {
  green: "good",
  yellow: "near",
  red: "expire",
}

// The inverse of SCAN_STATUS: which LED a received scan message lights.
export const MEDICINE_STATUS: Record<ScanStatus, MedicineStatus> = {
  good: "green",
  near: "yellow",
  expire: "red",
}

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

// `#7`, matching the live view's compartment hover label.
export function locationLabel(location: number): string {
  return `#${location}`
}

// The message that lights `medicine`'s compartment, or null when the medicine
// isn't placed in the tray and there's no LED to light.
export function toScanMessage(
  medicine: Medicine,
  thresholdDays: number,
  now: Date = new Date(),
): ScanMessage | null {
  if (medicine.location == null) return null

  return {
    type: SCAN_MESSAGE_TYPE,
    location: medicine.location,
    status: SCAN_STATUS[getMedicineStatus(medicine.expiration_date, thresholdDays, now)],
  }
}

function isScanStatus(value: unknown): value is ScanStatus {
  return typeof value === "string" && Object.hasOwn(MEDICINE_STATUS, value)
}

// The scan message in `data`, or null when it isn't one. Every client on
// apps/ws gets every message, so anything else is expected and ignored.
export function parseScanMessage(data: unknown): ScanMessage | null {
  if (typeof data !== "string") return null

  let parsed: unknown
  try {
    parsed = JSON.parse(data)
  } catch {
    return null
  }

  return toValidMessage(parsed, SCAN_MESSAGE_TYPE)
}

// `value` as an item of GET /medicines/locations, or null when it isn't one.
export function toLocationMessage(value: unknown): LocationMessage | null {
  return toValidMessage(value, LOCATION_MESSAGE_TYPE)
}

// `value` as a message of `expectedType` with a valid location and status, or
// null when it isn't one.
function toValidMessage<T extends string>(
  value: unknown,
  expectedType: T,
): { type: T; location: number; status: ScanStatus } | null {
  if (typeof value !== "object" || value === null) return null

  const { type, location, status } = value as Record<string, unknown>
  if (type !== expectedType) return null
  if (typeof location !== "number" || !LOCATIONS.includes(location)) return null
  if (!isScanStatus(status)) return null

  return { type: expectedType, location, status }
}
