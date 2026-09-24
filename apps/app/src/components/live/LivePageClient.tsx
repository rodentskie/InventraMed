"use client"

import { Box, Flex, Heading, HStack, Skeleton, Stack } from "@chakra-ui/react"
import { Status } from "@inventramed/snippets/status"
import dynamic from "next/dynamic"
import { useCallback, useState } from "react"
import {
  initialCompartments,
  LED_ORDER,
  MEDICINE_STATUS,
  statusToLeds,
} from "../../lib/live"
import type {
  CompartmentState,
  LedColor,
  LiveConnection,
  ScanMessage,
} from "../../types/live"
import { LedSimulationPanel } from "./LedSimulationPanel"
import { useScanSubscriber } from "./useScanSubscriber"

// Client-only: WebGL doesn't exist during SSR.
const LiveCanvas = dynamic(
  () => import("./LiveCanvas").then((m) => m.LiveCanvas),
  { ssr: false, loading: () => <Skeleton h="full" /> },
)

const LEGEND_LABEL: Record<LedColor, string> = {
  green: "Green — far from expiration",
  yellow: "Yellow — nearing expiration",
  red: "Red — expired",
}

const LEGEND_VALUE = {
  green: "success",
  yellow: "warning",
  red: "error",
} as const

const CONNECTION_LABEL: Record<LiveConnection, string> = {
  off: "Not connected",
  connecting: "Connecting…",
  live: "Live",
  disconnected: "Disconnected · retrying",
}

const CONNECTION_VALUE = {
  connecting: "warning",
  live: "success",
  disconnected: "error",
} as const

interface LivePageClientProps {
  // apps/ws URL to listen on; empty turns the connection off.
  wsUrl: string
}

export function LivePageClient({ wsUrl }: LivePageClientProps) {
  // The single place LED state lives. Both the WebSocket and the simulation
  // panel write here.
  const [compartments, setCompartments] = useState<CompartmentState[]>(initialCompartments)

  const setLed = useCallback((id: number, color: LedColor, on: boolean) => {
    setCompartments((prev) =>
      prev.some((c) => c.id === id && c.leds[color] !== on)
        ? prev.map((c) => (c.id === id ? { ...c, leds: { ...c.leds, [color]: on } } : c))
        : prev,
    )
  }, [])

  // Lights exactly the LED for the scanned status, so a rescan after a status
  // change replaces the old color. Other compartments are left as they are.
  const applyScan = useCallback((message: ScanMessage) => {
    const leds = statusToLeds(MEDICINE_STATUS[message.status])
    setCompartments((prev) =>
      prev.map((c) => (c.id === message.location ? { ...c, leds } : c)),
    )
  }, [])

  const connection = useScanSubscriber(wsUrl, applyScan)

  return (
    <Stack gap="6">
      <Flex align="center" gap="4" wrap="wrap">
        <Heading size="lg">Live View</Heading>
        {connection === "off" ? (
          <Status colorPalette="gray">{CONNECTION_LABEL.off}</Status>
        ) : (
          <Status value={CONNECTION_VALUE[connection]}>
            {CONNECTION_LABEL[connection]}
          </Status>
        )}
      </Flex>

      <Box h="70vh" minH="sm" borderWidth="1px" rounded="md" overflow="hidden">
        <LiveCanvas compartments={compartments} />
      </Box>

      <HStack gap="6" wrap="wrap">
        {LED_ORDER.map((color) => (
          <Status key={color} value={LEGEND_VALUE[color]}>
            {LEGEND_LABEL[color]}
          </Status>
        ))}
      </HStack>

      <LedSimulationPanel onLedChange={setLed} />
    </Stack>
  )
}
