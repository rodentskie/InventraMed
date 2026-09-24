"use client"

import { buttonGroup, folder, Leva, useControls } from "leva"
import {
  demoCompartments,
  initialCompartments,
  LED_ORDER,
  randomCompartments,
} from "../../lib/live"
import type { CompartmentState, LedColor } from "../../types/live"

// Hidden unless explicitly set to "false".
const HIDDEN = process.env.NEXT_PUBLIC_HIDE_LIVE_PRESETS !== "false"

interface LedSimulationPanelProps {
  onLedChange: (id: number, color: LedColor, on: boolean) => void
}

// Leva keys must be unique across the whole panel, not just per folder.
function ledKey(id: number, color: LedColor): string {
  return `c${id}-${color}`
}

function toLevaValues(compartments: CompartmentState[]): Record<string, boolean> {
  return Object.fromEntries(
    compartments.flatMap(({ id, leds }) =>
      LED_ORDER.map((color) => [ledKey(id, color), leds[color]]),
    ),
  )
}

// Stand-in for the WebSocket: toggles write into the same compartment state
// a live data source will. Shown only when NEXT_PUBLIC_HIDE_LIVE_PRESETS=false.
export function LedSimulationPanel({ onLedChange }: LedSimulationPanelProps) {
  const [, set] = useControls(() => ({
    Presets: buttonGroup({
      "All off": () => set(toLevaValues(initialCompartments())),
      Demo: () => set(toLevaValues(demoCompartments())),
      Random: () => set(toLevaValues(randomCompartments())),
    }),
    ...Object.fromEntries(
      initialCompartments().map(({ id }) => [
        `#${id}`,
        folder(
          Object.fromEntries(
            LED_ORDER.map((color) => [
              ledKey(id, color),
              {
                value: false,
                label: color,
                onChange: (on: boolean, _path: string, ctx: { initial: boolean }) => {
                  if (!ctx.initial) onLedChange(id, color, on)
                },
              },
            ]),
          ),
          { collapsed: true },
        ),
      ]),
    ),
  }))

  return <Leva hidden={HIDDEN} titleBar={{ title: "LED simulation" }} />
}
