"use client"

import { toaster } from "@inventramed/snippets/toaster"
import { useCallback } from "react"
import useWebSocket, { ReadyState } from "react-use-websocket"
import { toScanMessage } from "../../lib/live"
import type { ScanMessage } from "../../types/live"
import type { Medicine } from "../../types/medicine"

// The scanner page may stay open all day while apps/ws restarts, so it keeps
// retrying instead of giving up after the library's default 20 attempts.
const RECONNECT_INTERVAL_MS = 3000
const RECONNECT_ATTEMPTS = Infinity

// Connects to apps/ws while the scanner page is open and returns a function
// that sends a scanned medicine's scan message. An empty `wsUrl` turns the
// connection off, and publishing then does nothing. So does a null
// `thresholdDays` (settings not loaded): there is no reliable status to send.
export function useScanPublisher(
  wsUrl: string,
  thresholdDays: number | null,
): (medicine: Medicine) => void {
  const enabled = wsUrl !== ""
  const { sendJsonMessage, readyState } = useWebSocket<ScanMessage>(
    enabled ? wsUrl : null,
    {
      shouldReconnect: () => true,
      reconnectInterval: RECONNECT_INTERVAL_MS,
      reconnectAttempts: RECONNECT_ATTEMPTS,
    },
    enabled,
  )

  return useCallback(
    (medicine: Medicine) => {
      if (!enabled || thresholdDays == null) return

      const message = toScanMessage(medicine, thresholdDays)
      if (!message) return

      if (readyState !== ReadyState.OPEN) {
        toaster.create({
          type: "warning",
          title: "Couldn't update the tray LEDs",
          description: "The live server is unreachable. The scan result is still shown.",
        })
        return
      }

      // keep = false: a scan made while disconnected would light an LED later
      // for a medicine nobody is holding, so it is dropped, never queued.
      sendJsonMessage(message, false)
    },
    [enabled, thresholdDays, readyState, sendJsonMessage],
  )
}
