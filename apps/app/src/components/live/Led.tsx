"use client"

import { useMemo } from "react"
import { Color } from "three"
import { LED_COLORS } from "../../lib/live"
import type { LedColor } from "../../types/live"
import { LED_RADIUS } from "./dimensions"

// Above 1 so a lit LED crosses the Bloom luminance threshold and glows.
const ON_INTENSITY = 3
// How bright an unlit dome's color is, relative to its lit color.
const OFF_BRIGHTNESS = 0.25

interface LedProps {
  color: LedColor
  on: boolean
  position: [number, number, number]
}

export function Led({ color, on, position }: LedProps) {
  const litColor = LED_COLORS[color]
  const offColor = useMemo(
    () => new Color(litColor).multiplyScalar(OFF_BRIGHTNESS),
    [litColor],
  )

  // toneMapped stays off in both states: toggling it would force a shader
  // recompile, and an unlit dome's color is too dark to bloom anyway.
  return (
    <mesh position={position}>
      <sphereGeometry args={[LED_RADIUS, 16, 8, 0, Math.PI * 2, 0, Math.PI / 2]} />
      <meshStandardMaterial
        color={on ? litColor : offColor}
        emissive={litColor}
        emissiveIntensity={on ? ON_INTENSITY : 0}
        toneMapped={false}
      />
    </mesh>
  )
}
