### Live View — Phase 1 (3D Design)

This will be on `apps/app`.
A "live" 3D view of the physical InventraMed prototype: the medicine tray with its 12 compartments and the green/yellow/red LED cluster above each one. The end goal is for this page to connect to the WebSocket server (`apps/ws`, see @context/features/18-ws-server.spec.md) and light the virtual LEDs from live data, mirroring what the ESP32 shows on the real hardware.

**Phase 1 is only the design.** It builds the 3D model and makes every LED a controllable component, so a later phase only has to feed it data. Nothing connects to `apps/ws` in this phase. LED state comes from local React state, changed through a simulation panel (see "Simulation panel").

The model is based on the Tinkercad design in `context/screenshots/design/` (`front.png`, `top.png`, `left.png`, `right.png`, `back.png`).

## The design (from the screenshots)

- **Body**: an open-top box with a roughly square footprint. The back wall is tall, and the top drops in a **slope toward the front**, so the side walls are trapezoids (`left.png`/`right.png`). The back is a plain flat wall (`back.png`)
- **Rim**: the outer walls rise slightly above the sloped panel, so the panel sits recessed inside a rim. At the back there is a flat strip along the top of the back wall (`front.png`/`top.png`)
- **Compartment panel**: the sloped surface holds a **3 × 4 grid of 12 square pockets** recessed into the panel, with the openings facing up along the slope
- **LEDs**: above each pocket (on the uphill side) sits a row of **3 small dome LEDs, ordered green, yellow, red from left to right** as seen from the front
- **Handles**: one on each side wall (left and right), near the top and toward the back. Each is a horizontal bar with angled ends, standing off from the wall
- **Feet**: 4 short cylinders, one under each corner

Proportions are estimated from the screenshots, which have no dimensions on them. All sizes live in one constants file (see Components) so they can be tuned against the screenshots without touching the component code. Starting values, relative to the footprint width `W = 1`:

| Part | Approx. size |
|------|--------------|
| Footprint (width × depth) | `1 × 1.05` |
| Back wall height | `0.6` |
| Front wall height | `0.2` |
| Wall thickness | `0.03` |
| Rim above panel | `0.03` |
| Pocket (square opening × depth) | `0.17 × 0.1` |
| Gap between pockets | `0.06` |
| LED dome radius | `0.012`, with centers `0.03` apart |
| Foot (radius × height) | `0.03 × 0.03` |

## Libraries

Checked against current docs (Context7). New packages go in the root `package.json` `dependencies`. This is a single-`package.json` Nx workspace, same as the other `apps/app` packages. Install them with yarn, not by editing `package.json` by hand.

### Added in this phase

| Package | Why |
|---------|-----|
| `three` | The WebGL engine everything else renders through |
| `@types/three` (devDependency) | TypeScript types for `three` (strict mode, no `any`) |
| `@react-three/fiber` **v9** | React renderer for three.js. **v9 is required** because it's the major that pairs with React 19 (v8 pairs with React 18). Provides `<Canvas>`, JSX meshes/materials, and `useFrame` |
| `@react-three/drei` **v10** (the fiber-9 line) | Helpers so we don't hand-roll them: `OrbitControls` (rotate/zoom/pan the model), `RoundedBox` (soft edges on the body and handles), `ContactShadows` (a ground shadow like Tinkercad's), `Html` (hover labels on compartments, optional), `Grid` (optional workplane grid) |
| `@react-three/postprocessing` **v3** (the fiber-9 line) | `EffectComposer` + `Bloom`, so a lit LED actually *glows* instead of just being a brighter color. Bloom is selective by threshold: with `luminanceThreshold={1}`, only materials with `emissiveIntensity > 1` and `toneMapped={false}` bloom. That fits exactly: lit LEDs glow and nothing else does. No `Selection`/`SelectiveBloom` wrapper is needed |
| `@react-three/csg` | Constructive solid geometry (`Geometry` / `Base` / `Subtraction`), built on `three-bvh-csg`. Used to **cut the 12 pockets out of the sloped panel** and to hollow the body. The alternative is assembling the panel from dozens of thin boxes around each hole, which is harder to keep aligned when dimensions change. The CSG result is computed once and memoized, since the geometry never changes at runtime; only LED materials do |
| `leva` | A small GUI panel (`useControls`) for the phase-1 simulation panel: per-LED on/off toggles grouped in collapsible folders per compartment. Hidden by default through `NEXT_PUBLIC_HIDE_LIVE_PRESETS` (see "Simulation panel"), so it is a testing tool and not user-facing UI |

### For the next phase (not installed now)

| Package | Why |
|---------|-----|
| `react-use-websocket` | A React hook around `WebSocket` with automatic reconnect (`shouldReconnect`, `reconnectInterval`, `reconnectAttempts`), a `readyState` for a connection badge, `lastJsonMessage` for parsed payloads, heartbeat support, and a `shouldConnect` flag to pause the connection. Its v4 line supports React 18+. The alternative is a small custom `useWsLeds` hook over the native `WebSocket`. Decide when the message contract is designed |

### Considered, not used

- `@react-three/rapier` / `cannon`: physics isn't needed
- `drei`'s `Environment` with an HDR preset: it downloads the HDR from a CDN at runtime. Plain `ambientLight` + `directionalLight` are enough for a matte grey model and keep the page working offline and on the LAN
- `@react-three/xr`: AR/VR is out of scope
- Exporting the Tinkercad model as `.glb` and loading it with drei's `useGLTF` (with `gltfjsx` to generate a typed component). This is a real alternative to building the model from primitives. It matches the design exactly and saves the modeling work, but Tinkercad exports would need the LEDs split out as separately named meshes to stay controllable, and every design change means re-exporting. **Phase 1 builds from primitives**, so the LEDs are React components from the start. Revisit if the physical design gets more detailed

## Requirements

### Routing & layout

- `/live`, at `app/(app)/live/page.tsx`, **inside** the guarded `(app)` route group, so it gets the `SideNav`/`TopNav` chrome and needs login. Add `"/live"` and `"/live/:path*"` to the `proxy.ts` matcher
- Add `{ label: "Live View", href: "/live" }` to `NAV_ITEMS` in `src/lib/nav.ts`, after "Scanner"
- Heading "Live View", with a status badge beside it. In phase 1 the badge always reads **"Design preview · not connected"** (the `Status` snippet, neutral color). The next phase turns this into the real connection state
- Below it, the 3D canvas at full content width, about `70vh` tall (`minH` so it stays usable on phones)
- Below the canvas, a legend using the `Status` snippet: Green = far from expiration, Yellow = nearing expiration, Red = expired (same wording as @context/project-overview.md)

### Canvas & scene

- The `<Canvas>` lives in a client component loaded with `next/dynamic(..., { ssr: false })`, the same pattern as `BarcodeCameraScanner`, because WebGL doesn't exist on the server
- `frameloop="demand"`: the scene is static apart from LED changes, so it only re-renders on input (OrbitControls invalidates on its own) or when LED props change. This keeps the GPU idle on a page that may stay open all day
- `dpr={[1, 2]}` and `shadows` enabled
- The camera starts at the front-top three-quarter angle of `front.png`, looking down the slope at the compartments
- `OrbitControls`:
  - Damping on and panning off
  - Polar angle clamped so you can't go under the floor
  - Zoom clamped to a sensible min/max distance
- Lighting: one `ambientLight` plus one `directionalLight` that casts shadows. `ContactShadows` under the model
- Background follows the color mode. The canvas is transparent (`gl={{ alpha: true }}`) and the page's Chakra background shows through, so dark mode works with no scene changes
- Body material: a matte light grey `meshStandardMaterial`, close to the Tinkercad look. Pockets are slightly darker inside, so they read as recesses
- `Canvas` `fallback` prop: when WebGL isn't available, show "Your browser can't display the 3D view (WebGL unavailable)." via the `Alert` snippet

### LED component (the controllable part)

The LED is the key piece of this feature. Each physical LED is one `<Led>` component whose on/off state is a **prop**, so any data source can drive it: the leva panel now, the WebSocket later.

```ts
// src/types/live.ts
import type { MedicineStatus } from "../lib/medicine-status"

export type LedColor = MedicineStatus // "green" | "yellow" | "red"

export type LedState = Record<LedColor, boolean> // which LEDs in a cluster are on

export interface CompartmentState {
  id: number // 1–12, row-major from the back-left pocket as seen from the front
  leds: LedState
}
```

- `Led` props: `color: LedColor`, `on: boolean`, `position: [number, number, number]`
  - **On**: `meshStandardMaterial` with `emissive` set to the LED color, `emissiveIntensity` about `3`, and `toneMapped={false}`, so it crosses the bloom threshold and glows
  - **Off**: same color darkened, `emissiveIntensity={0}`, tone-mapped as normal. It reads as an unlit colored dome, like the real LED
  - Geometry is a half-sphere dome (`sphereGeometry` with `thetaLength = π/2`) sitting on the panel surface
  - No per-LED `pointLight`. 36 real lights would be expensive, and bloom already sells the glow
- The LED colors are named constants (`LED_COLORS`), not inline hex values, and are brighter than the Chakra status palette so they read as light sources
- `LedCluster` renders the 3 `Led`s for one compartment in fixed green → yellow → red order. Props: `leds: LedState`, `position`
- Each LED is independent. The component doesn't enforce "only one on", because the hardware might blink or light several (e.g. a startup self-test). A helper `statusToLeds(status: MedicineStatus | null): LedState` in `src/lib/live.ts` covers the normal case of lighting exactly the LED that matches a status, or none when `null`

### Compartment component

- `Compartment` props: `state: CompartmentState`, plus its grid `row`/`col`. It places its `LedCluster` on the panel just uphill of its pocket, using the grid position
- On hover, the pocket outline highlights and a drei `Html` label shows `#<id>` (e.g. `#7`). This makes it easy to match a virtual compartment to a physical one while testing
- Pockets are cut into the panel with CSG in `CompartmentPanel`, not by `Compartment` itself. `Compartment` only positions the LEDs and the hover target, so each component has one job

### State ownership

- `LivePageClient` owns `compartments: CompartmentState[]` (12 entries, all LEDs off initially) and passes each one down. This is the single place a future data source writes to
- An initial state and a deterministic demo preset live in `src/lib/live.ts`:
  - `initialCompartments()`: all off
  - `demoCompartments()`: a mix of green/yellow/red and one empty compartment, so the screenshot/demo looks alive

### Simulation panel (phase 1 only, dev-only)

- `LedSimulationPanel` uses `leva`'s `useControls`:
  - One collapsed folder per compartment (`#1` … `#12`), each with `green` / `yellow` / `red` boolean toggles
  - A top-level button group: "All off", "Demo", and "Random" (each compartment gets a random status via `statusToLeds`)
- Toggle changes write into `LivePageClient`'s `compartments` state, which is the same path the WebSocket will use later
- Visibility is controlled by `NEXT_PUBLIC_HIDE_LIVE_PRESETS` (`apps/app/.env`/`.env.example`, boolean, **defaults to `true`**). The panel is hidden unless the value is exactly `false`, in any environment. It must be `NEXT_PUBLIC_` because the panel renders client-side. When hidden, the page renders the design with all LEDs off

### Components

Under `src/components/live/`:

- `LivePageClient.tsx`: owns compartment state, renders the status badge, canvas and legend, and mounts the simulation panel
- `LiveCanvas.tsx`: the dynamically imported `<Canvas>`. Sets up camera, lights, `OrbitControls`, `ContactShadows` and the `EffectComposer`/`Bloom`, and renders `TrayModel`
- `TrayModel.tsx`: the body (walls, rim, back strip), both handles and the feet, plus `CompartmentPanel`
- `CompartmentPanel.tsx`: the sloped panel with the 12 pockets cut out via `@react-three/csg` (memoized), and the 12 `Compartment`s positioned along the slope
- `Compartment.tsx`, `LedCluster.tsx`, `Led.tsx`: as described above
- `LedSimulationPanel.tsx`: the leva controls
- `dimensions.ts`: every size and position constant from the table above (`SCREAMING_SNAKE_CASE`), plus derived values such as the slope angle and pocket grid origin. No magic numbers in the components

Also:

- `src/types/live.ts`: `LedColor`, `LedState`, `CompartmentState`
- `src/lib/live.ts`: `statusToLeds`, `initialCompartments`, `demoCompartments`, `LED_COLORS`

### Next.js config

- Only if the build complains about ESM in `three` or its addons, add `transpilePackages: ["three"]` to `apps/app/next.config.js`. Recent Next versions usually don't need it, so try without it first

### Testing

- `apps/app` has no unit-test runner set up. This phase is verified with `lint` and `build` via Nx, plus a manual check:
  - The model matches the 5 screenshot angles when orbited
  - Toggling each LED in leva lights only that dome, and lit LEDs glow
  - Dark and light mode both look right
  - The WebGL fallback shows when WebGL is disabled in the browser
- `statusToLeds` is pure logic worth a unit test once an app test runner exists (out of scope here)

## Next phase (not in this spec, for context only)

- Connect to `apps/ws` from `LivePageClient` and write incoming messages into `compartments`, replacing the leva panel as the data source. The panel can stay for dev
- Define the message contract. `apps/ws` relays opaque bytes and doesn't care. A starting point for discussion:

  ```json
  { "type": "led", "compartment": 7, "leds": { "green": false, "yellow": true, "red": false } }
  ```

  plus a full-snapshot message, since `apps/ws` has no replay and a late-joining page otherwise starts blank
- Turn the "not connected" badge into the real connection state (`connecting` / `live` / `disconnected`)
- A `NEXT_PUBLIC_WS_URL` env var (the browser connects directly), and adding the app's origin to `WS_ALLOWED_ORIGINS` on `apps/ws`
- Mapping compartments to medicines (which medicine sits in which pocket) is a data-model question for a later feature

## Out of scope

- Any WebSocket connection or message handling
- ESP32 firmware
- Any API or database change (no compartment ↔ medicine mapping yet)
- Loading an exported Tinkercad `.glb` (see "Considered, not used")
- Animations beyond the LED glow (no blinking, no camera tours)

## Open questions

- Should production show all LEDs off, or the `demoCompartments()` preset, until the WebSocket phase lands?
- Should `/live` be public like `/scanner` (e.g. shown on a wall display without login), or stay behind login as specced here?

## References

- @context/project-overview.md
- @context/features/17-scanner.spec.md (`next/dynamic` + `ssr: false` pattern)
- @context/features/18-ws-server.spec.md
- @context/screenshots/design/front.png
- @context/screenshots/design/top.png
- @context/screenshots/design/left.png
- @context/screenshots/design/right.png
- @context/screenshots/design/back.png
- @apps/app/src/lib/medicine-status.ts
- @apps/app/src/lib/nav.ts
- @apps/app/src/proxy.ts
- @apps/app/src/components/scanner/BarcodeCameraScanner.tsx
- @libs/ts/snippets/src/lib/status.tsx
- @libs/ts/snippets/src/lib/alert.tsx
- https://r3f.docs.pmnd.rs/getting-started/installation
- https://drei.docs.pmnd.rs
- https://react-postprocessing.docs.pmnd.rs/effects/bloom
- https://github.com/pmndrs/react-three-csg
- https://github.com/pmndrs/leva
- https://github.com/robtaussig/react-use-websocket
