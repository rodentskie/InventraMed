"use client"

import { Alert } from "@inventramed/snippets/alert"
import { useColorModeValue } from "@inventramed/snippets/color-mode"
import { ContactShadows, OrbitControls } from "@react-three/drei"
import { Canvas } from "@react-three/fiber"
import { Bloom, EffectComposer } from "@react-three/postprocessing"
import type { CompartmentState } from "../../types/live"
import { CAMERA_POSITION, CAMERA_TARGET } from "./dimensions"
import { TrayModel } from "./TrayModel"

// Match Chakra's `bg` token in each color mode.
const LIGHT_BACKGROUND = "#ffffff"
const DARK_BACKGROUND = "#09090b"
const CAMERA_FOV = 40
const MIN_DISTANCE = 2.5
const MAX_DISTANCE = 8
// Stop just short of horizontal so the camera can't orbit under the floor.
const MAX_POLAR_ANGLE = Math.PI / 2 - 0.05
const SHADOW_MAP_SIZE = 1024

interface LiveCanvasProps {
  compartments: CompartmentState[]
}

export function LiveCanvas({ compartments }: LiveCanvasProps) {
  const background = useColorModeValue(LIGHT_BACKGROUND, DARK_BACKGROUND)

  return (
    // "demand": the scene is static apart from LED changes, so only render on
    // input or prop changes instead of every frame.
    <Canvas
      frameloop="demand"
      dpr={[1, 2]}
      shadows
      camera={{ position: CAMERA_POSITION, fov: CAMERA_FOV }}
      fallback={
        <Alert
          status="warning"
          title="Your browser can't display the 3D view (WebGL unavailable)."
        />
      }
    >
      <color attach="background" args={[background]} />
      <ambientLight intensity={0.8} />
      <directionalLight
        position={[3, 5, 4]}
        intensity={1.8}
        castShadow
        shadow-mapSize={[SHADOW_MAP_SIZE, SHADOW_MAP_SIZE]}
      />
      <TrayModel compartments={compartments} />
      <ContactShadows position={[0, 0, 0]} opacity={0.45} scale={5} blur={2} far={2} />
      <OrbitControls
        makeDefault
        target={CAMERA_TARGET}
        enableDamping
        enablePan={false}
        minDistance={MIN_DISTANCE}
        maxDistance={MAX_DISTANCE}
        maxPolarAngle={MAX_POLAR_ANGLE}
      />
      {/* Only materials pushed above 1 (lit LEDs) cross the threshold. */}
      <EffectComposer>
        <Bloom mipmapBlur luminanceThreshold={1} intensity={1.2} />
      </EffectComposer>
    </Canvas>
  )
}
