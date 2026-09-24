"use client"

import { useEffect, useRef } from "react"
import useWebSocket, { ReadyState } from "react-use-websocket"
import { parseScanMessage } from "../../lib/live"
import type { LiveConnection, ScanMessage } from "../../types/live"

// Same as the scanner: /live may stay open all day on a display while
// apps/ws restarts, so it keeps retrying.
const RECONNECT_INTERVAL_MS = 3000
const RECONNECT_ATTEMPTS = Infinity

const CONNECTION: Record<ReadyState, LiveConnection> = {
  [ReadyState.UNINSTANTIATED]: "connecting",
  [ReadyState.CONNECTING]: "connecting",
  [ReadyState.OPEN]: "live",
  [ReadyState.CLOSING]: "disconnected",
  [ReadyState.CLOSED]: "disconnected",
}

// Listens to apps/ws and calls `onScan` for every valid scan message. An
// empty `wsUrl` turns the connection off. Returns the connection state.
export function useScanSubscriber(
  wsUrl: string,
  onScan: (message: ScanMessage) => void,
): LiveConnection {
  const enabled = wsUrl !== ""

  // Read through a ref so a new callback never reconnects the socket.
  const onScanRef = useRef(onScan)
  useEffect(() => {
    onScanRef.current = onScan
  }, [onScan])

  const { readyState } = useWebSocket(
    enabled ? wsUrl : null,
    {
      shouldReconnect: () => true,
      reconnectInterval: RECONNECT_INTERVAL_MS,
      reconnectAttempts: RECONNECT_ATTEMPTS,
      onMessage: (event) => {
        const message = parseScanMessage(event.data)
        if (message) onScanRef.current(message)
      },
      // onMessage runs before filter, so every message is still handled, but
      // lastMessage never updates and the page doesn't re-render per message.
      filter: () => false,
    },
    enabled,
  )

  return enabled ? CONNECTION[readyState] : "off"
}
