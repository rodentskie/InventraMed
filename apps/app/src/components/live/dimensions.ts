import { COMPARTMENT_COUNT } from "../../lib/live"

// Scene units. Proportions are estimated from context/screenshots/design/*.png,
// which carry no measurements — tune the model here, not in the components.
// Axes: +x is right and +z is the front, as seen from the front; +y is up.

export const BODY_WIDTH = 2
export const BODY_DEPTH = 2.1
export const BACK_HEIGHT = 1.2
export const FRONT_HEIGHT = 0.4
export const WALL_THICKNESS = 0.06
// Thicker than the other walls: its top is the flat strip along the back.
export const BACK_WALL_THICKNESS = 0.12
// How far from the back (as a fraction of the depth) the side walls stay flat
// before sloping down to the front.
export const RIDGE_DEPTH_FRACTION = 0.45
// How far the front wall rises above the compartment panel.
export const RIM_HEIGHT = 0.06

export const PANEL_BACK_HEIGHT = 0.9
export const PANEL_THICKNESS = 0.26

export const GRID_COLS = 4
export const GRID_ROWS = COMPARTMENT_COUNT / GRID_COLS
export const POCKET_SIZE = 0.34
export const POCKET_DEPTH = 0.2
export const POCKET_GAP = 0.12
// Strip uphill of each pocket that holds its LED cluster.
export const LED_BAND = 0.1

export const LED_RADIUS = 0.024
export const LED_SPACING = 0.06

export const FOOT_RADIUS = 0.06
export const FOOT_HEIGHT = 0.06
export const FOOT_INSET = 0.14

export const HANDLE_LENGTH = 0.8
export const HANDLE_REACH = 0.14
export const HANDLE_THICKNESS = 0.07
// Fraction of the bar's length taken by the angled end on each side.
export const HANDLE_TAPER_FRACTION = 0.15
export const HANDLE_HEIGHT = BACK_HEIGHT * 0.65
export const HANDLE_Z = -BODY_DEPTH / 2 + BODY_DEPTH * 0.28

// Derived

export const INNER_WIDTH = BODY_WIDTH - 2 * WALL_THICKNESS
export const RIDGE_Z = -BODY_DEPTH / 2 + BODY_DEPTH * RIDGE_DEPTH_FRACTION

const PANEL_FRONT = { y: FRONT_HEIGHT - RIM_HEIGHT, z: BODY_DEPTH / 2 - WALL_THICKNESS }
const PANEL_BACK = { y: PANEL_BACK_HEIGHT, z: -BODY_DEPTH / 2 + BACK_WALL_THICKNESS }

// Tilt about the x axis that raises the panel's back edge.
export const PANEL_ANGLE = Math.atan2(PANEL_BACK.y - PANEL_FRONT.y, PANEL_FRONT.z - PANEL_BACK.z)
export const PANEL_LENGTH = Math.hypot(PANEL_BACK.y - PANEL_FRONT.y, PANEL_FRONT.z - PANEL_BACK.z)
// Center of the panel's top surface.
export const PANEL_CENTER: [number, number, number] = [
  0,
  (PANEL_FRONT.y + PANEL_BACK.y) / 2,
  (PANEL_FRONT.z + PANEL_BACK.z) / 2,
]

const COL_PITCH = POCKET_SIZE + POCKET_GAP
const ROW_PITCH = LED_BAND + POCKET_SIZE + POCKET_GAP

// A pocket's center on the panel's top surface, in panel-local [x, z]:
// x runs across the panel, z runs down the slope (row 0 is the back row).
// Each row is an LED band followed by the pocket, centered as one block.
export function pocketCenter(row: number, col: number): [number, number] {
  return [
    (col - (GRID_COLS - 1) / 2) * COL_PITCH,
    (row - (GRID_ROWS - 1) / 2) * ROW_PITCH + LED_BAND / 2,
  ]
}

export const CAMERA_POSITION: [number, number, number] = [0, 2.8, 3.6]
export const CAMERA_TARGET: [number, number, number] = [0, 0.5, 0]
