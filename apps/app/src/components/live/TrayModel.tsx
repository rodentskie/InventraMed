"use client"

import { RoundedBox } from "@react-three/drei"
import { useMemo } from "react"
import { Shape } from "three"
import type { CompartmentState } from "../../types/live"
import { CompartmentPanel } from "./CompartmentPanel"
import {
  BACK_HEIGHT,
  BACK_WALL_THICKNESS,
  BODY_DEPTH,
  BODY_WIDTH,
  FOOT_HEIGHT,
  FOOT_INSET,
  FOOT_RADIUS,
  FRONT_HEIGHT,
  HANDLE_HEIGHT,
  HANDLE_LENGTH,
  HANDLE_REACH,
  HANDLE_TAPER_FRACTION,
  HANDLE_THICKNESS,
  HANDLE_Z,
  INNER_WIDTH,
  RIDGE_Z,
  WALL_THICKNESS,
} from "./dimensions"

const BODY_COLOR = "#d4d4d8"
const HANDLE_COLOR = "#a1a1aa"
const FOOT_COLOR = "#71717a"
const HANDLE_RADIUS = 0.015

// Side profile in the wall's shape space: x = -z (front on the left), y = up.
function sideProfile(): Shape {
  const shape = new Shape()
  shape.moveTo(-BODY_DEPTH / 2, 0)
  shape.lineTo(-BODY_DEPTH / 2, FRONT_HEIGHT)
  shape.lineTo(-RIDGE_Z, BACK_HEIGHT)
  shape.lineTo(BODY_DEPTH / 2, BACK_HEIGHT)
  shape.lineTo(BODY_DEPTH / 2, 0)
  shape.closePath()
  return shape
}

// A trapezoid side wall: flat along the back, sloping down to the front.
function SideWall({ x }: { x: number }) {
  const shape = useMemo(sideProfile, [])
  return (
    // Rotating by +90° about y maps the shape's x to -z and extrudes along +x.
    <mesh position={[x, 0, 0]} rotation={[0, Math.PI / 2, 0]} castShadow receiveShadow>
      <extrudeGeometry args={[shape, { depth: WALL_THICKNESS, bevelEnabled: false }]} />
      <meshStandardMaterial color={BODY_COLOR} />
    </mesh>
  )
}

const BAR_LENGTH = HANDLE_LENGTH * (1 - 2 * HANDLE_TAPER_FRACTION)
const TAPER_RUN = HANDLE_LENGTH * HANDLE_TAPER_FRACTION
const STRUT_LENGTH = Math.hypot(HANDLE_REACH, TAPER_RUN)
const STRUT_ANGLE = Math.atan2(TAPER_RUN, HANDLE_REACH)

// A bar standing off the wall on two angled struts. Built pointing +x;
// `side` mirrors it onto the left wall.
function Handle({ side }: { side: 1 | -1 }) {
  return (
    <group
      position={[(side * BODY_WIDTH) / 2, HANDLE_HEIGHT, HANDLE_Z]}
      rotation={[0, side === 1 ? 0 : Math.PI, 0]}
    >
      <RoundedBox
        args={[HANDLE_THICKNESS, HANDLE_THICKNESS, BAR_LENGTH]}
        radius={HANDLE_RADIUS}
        position={[HANDLE_REACH, 0, 0]}
        castShadow
      >
        <meshStandardMaterial color={HANDLE_COLOR} />
      </RoundedBox>
      {([1, -1] as const).map((end) => (
        <RoundedBox
          key={end}
          args={[STRUT_LENGTH, HANDLE_THICKNESS, HANDLE_THICKNESS]}
          radius={HANDLE_RADIUS}
          position={[HANDLE_REACH / 2, 0, (end * (HANDLE_LENGTH / 2 + BAR_LENGTH / 2)) / 2]}
          rotation={[0, end * STRUT_ANGLE, 0]}
          castShadow
        >
          <meshStandardMaterial color={HANDLE_COLOR} />
        </RoundedBox>
      ))}
    </group>
  )
}

const FOOT_POSITIONS = [
  [-1, -1],
  [-1, 1],
  [1, -1],
  [1, 1],
].map(([sx, sz]): [number, number, number] => [
  sx * (BODY_WIDTH / 2 - FOOT_INSET),
  FOOT_HEIGHT / 2,
  sz * (BODY_DEPTH / 2 - FOOT_INSET),
])

function Box({ size, position }: { size: [number, number, number]; position: [number, number, number] }) {
  return (
    <mesh position={position} castShadow receiveShadow>
      <boxGeometry args={size} />
      <meshStandardMaterial color={BODY_COLOR} />
    </mesh>
  )
}

interface TrayModelProps {
  compartments: CompartmentState[]
}

// The whole prototype, resting on its feet at y = 0.
export function TrayModel({ compartments }: TrayModelProps) {
  return (
    <group>
      {FOOT_POSITIONS.map((position) => (
        <mesh key={position.join(":")} position={position} castShadow>
          <cylinderGeometry args={[FOOT_RADIUS, FOOT_RADIUS, FOOT_HEIGHT, 24]} />
          <meshStandardMaterial color={FOOT_COLOR} />
        </mesh>
      ))}
      <group position={[0, FOOT_HEIGHT, 0]}>
        <Box
          size={[INNER_WIDTH, WALL_THICKNESS, BODY_DEPTH]}
          position={[0, WALL_THICKNESS / 2, 0]}
        />
        <Box
          size={[INNER_WIDTH, FRONT_HEIGHT, WALL_THICKNESS]}
          position={[0, FRONT_HEIGHT / 2, BODY_DEPTH / 2 - WALL_THICKNESS / 2]}
        />
        <Box
          size={[INNER_WIDTH, BACK_HEIGHT, BACK_WALL_THICKNESS]}
          position={[0, BACK_HEIGHT / 2, -BODY_DEPTH / 2 + BACK_WALL_THICKNESS / 2]}
        />
        <SideWall x={-BODY_WIDTH / 2} />
        <SideWall x={BODY_WIDTH / 2 - WALL_THICKNESS} />
        <Handle side={1} />
        <Handle side={-1} />
        <CompartmentPanel compartments={compartments} />
      </group>
    </group>
  )
}
