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

// Wire value of a scan message's status: which LED the receiver lights.
export type ScanStatus = "good" | "near" | "expire"

// Sent to apps/ws by the scanner page after a successful lookup. Receivers
// check `type` first and ignore types they don't recognise.
export interface ScanMessage {
  type: "scan"
  // 1–12, same numbering as CompartmentState.id.
  location: number
  status: ScanStatus
}

// State of /live's connection to apps/ws. "off" means WS_SERVER is unset.
export type LiveConnection = "off" | "connecting" | "live" | "disconnected"
