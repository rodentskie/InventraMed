"use client"

import { Edges, Html } from "@react-three/drei"
import type { ThreeEvent } from "@react-three/fiber"
import { useCallback, useState } from "react"
import type { CompartmentState } from "../../types/live"
import styles from "./Compartment.module.css"
import { LED_BAND, POCKET_SIZE, pocketCenter } from "./dimensions"
import { LedCluster } from "./LedCluster"

const HOVER_COLOR = "#3b82f6"
// Lifts the hover target just off the panel surface to avoid z-fighting.
const SURFACE_OFFSET = 0.002
const LABEL_HEIGHT = 0.08

interface CompartmentProps {
  state: CompartmentState
  row: number
  col: number
}

// Positions one compartment's LED cluster and hover target on the panel.
// The pocket itself is cut out by CompartmentPanel.
export function Compartment({ state, row, col }: CompartmentProps) {
  const [hovered, setHovered] = useState(false)
  const [x, z] = pocketCenter(row, col)

  const handleOver = useCallback((e: ThreeEvent<PointerEvent>) => {
    e.stopPropagation()
    setHovered(true)
  }, [])
  const handleOut = useCallback(() => setHovered(false), [])

  return (
    <group>
      <LedCluster
        leds={state.leds}
        position={[x, 0, z - POCKET_SIZE / 2 - LED_BAND / 2]}
      />
      <mesh
        position={[x, SURFACE_OFFSET, z]}
        rotation={[-Math.PI / 2, 0, 0]}
        onPointerOver={handleOver}
        onPointerOut={handleOut}
      >
        <planeGeometry args={[POCKET_SIZE, POCKET_SIZE]} />
        <meshBasicMaterial transparent opacity={0} depthWrite={false} />
        {hovered && <Edges color={HOVER_COLOR} lineWidth={2} />}
      </mesh>
      {hovered && (
        <Html position={[x, LABEL_HEIGHT, z]} center>
          <span className={styles.label}>#{state.id}</span>
        </Html>
      )}
    </group>
  )
}
