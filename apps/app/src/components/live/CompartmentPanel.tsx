"use client"

import { Base, Geometry, Subtraction } from "@react-three/csg"
import { memo } from "react"
import type { CompartmentState } from "../../types/live"
import { Compartment } from "./Compartment"
import {
  GRID_COLS,
  GRID_ROWS,
  INNER_WIDTH,
  PANEL_ANGLE,
  PANEL_CENTER,
  PANEL_LENGTH,
  PANEL_THICKNESS,
  POCKET_DEPTH,
  POCKET_SIZE,
  pocketCenter,
} from "./dimensions"

const PANEL_COLOR = "#e4e4e7"
// Darker than the panel so the pockets read as recesses.
const POCKET_COLOR = "#a1a1aa"
// Cutters poke above the surface so they don't share a face with the panel.
const CUT_OVERSHOOT = 0.02

const POCKETS = Array.from({ length: GRID_ROWS * GRID_COLS }, (_, i) =>
  pocketCenter(Math.floor(i / GRID_COLS), i % GRID_COLS),
)

// Memoized with no props: the CSG result never changes at runtime, so LED
// updates must not recompute it.
const PanelSlab = memo(function PanelSlab() {
  return (
    <mesh castShadow receiveShadow>
      <Geometry useGroups>
        <Base position={[0, -PANEL_THICKNESS / 2, 0]}>
          <boxGeometry args={[INNER_WIDTH, PANEL_THICKNESS, PANEL_LENGTH]} />
          <meshStandardMaterial color={PANEL_COLOR} />
        </Base>
        {POCKETS.map(([x, z]) => (
          <Subtraction
            key={`${x}:${z}`}
            position={[x, (CUT_OVERSHOOT - POCKET_DEPTH) / 2, z]}
          >
            <boxGeometry args={[POCKET_SIZE, POCKET_DEPTH + CUT_OVERSHOOT, POCKET_SIZE]} />
            <meshStandardMaterial color={POCKET_COLOR} />
          </Subtraction>
        ))}
      </Geometry>
    </mesh>
  )
})

interface CompartmentPanelProps {
  compartments: CompartmentState[]
}

// The sloped panel. Children are laid out in panel-local space, where y = 0
// is the top surface and -z is uphill (toward the back).
export function CompartmentPanel({ compartments }: CompartmentPanelProps) {
  return (
    <group position={PANEL_CENTER} rotation={[PANEL_ANGLE, 0, 0]}>
      <PanelSlab />
      {compartments.map((compartment, i) => (
        <Compartment
          key={compartment.id}
          state={compartment}
          row={Math.floor(i / GRID_COLS)}
          col={i % GRID_COLS}
        />
      ))}
    </group>
  )
}
