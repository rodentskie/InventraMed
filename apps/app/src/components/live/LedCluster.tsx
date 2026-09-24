"use client"

import { LED_ORDER } from "../../lib/live"
import type { LedState } from "../../types/live"
import { LED_SPACING } from "./dimensions"
import { Led } from "./Led"

interface LedClusterProps {
  leds: LedState
  position: [number, number, number]
}

export function LedCluster({ leds, position }: LedClusterProps) {
  const middle = (LED_ORDER.length - 1) / 2
  return (
    <group position={position}>
      {LED_ORDER.map((color, i) => (
        <Led
          key={color}
          color={color}
          on={leds[color]}
          position={[(i - middle) * LED_SPACING, 0, 0]}
        />
      ))}
    </group>
  )
}
