### Scanner Page

This will be on `apps/app`.
A webcam-based barcode scanner page: point the camera at a medicine's barcode, decode it in the browser, and look it up via `GET /medicines/barcode/{barcode}` to show its details. Per @context/project-overview.md, no physical barcode scanner hardware is used — scanning happens through the website.

**Public, not behind login.** Both the `/scanner` page and `GET /medicines/barcode/{barcode}` are unauthenticated — someone can scan/look up a medicine without signing in. This means:

- `apps/api`: `GET /medicines/barcode/{barcode}` is registered without the `auth(...)` middleware wrapper in `cmd/api/main.go` (same pattern as `POST /login`); `security` is removed from that operation in the swagger spec, and its `401` response is dropped. The handler itself also had its own "fail closed" guard (`h.caller`, shared by every `medicine` handler) that 401s whenever there's no authenticated account in context, independently of the `main.go` wrapper — that guard had to be removed from `GetByBarcode` specifically (it never used the caller's identity anyway) for the route to actually be public. See `internal/handler/medicine/list.go` and the `TestNoCallerInContext`/`TestGetByBarcode_NoCallerInContext` tests in that package
- `apps/app`: `/scanner` is **not** listed in `proxy.ts`'s `matcher`, and the page lives outside the `(app)` guarded route group (its own top-level route, `app/scanner/page.tsx`, not `app/(app)/scanner/page.tsx`) so it doesn't inherit the authenticated `SideNav`/`TopNav` chrome. It gets a minimal header of its own (brand mark linking to `/`, color mode toggle)
- The `NAV_ITEMS` entry still links to it for convenience when browsing while logged in

Two new packages are needed, since they do different jobs:

- [`react-barcode-scanner`](https://www.npmjs.com/package/react-barcode-scanner) — reads a barcode from the webcam feed (native Barcode Detection API with a WASM polyfill fallback). This is the actual "scanning."
- [`react-barcode`](https://www.npmjs.com/package/react-barcode) — renders a barcode as an SVG from a text value (wraps JsBarcode). It cannot read a camera feed; here it's only used to redraw the matched medicine's barcode on the result card as a visual confirmation.

## Requirements

### Dependencies

- Add `react-barcode` and `react-barcode-scanner` to the root `package.json` `dependencies` (this is a single-`package.json` Nx workspace, same as the other `apps/app` packages)
- `react-barcode-scanner` requires a secure context (HTTPS or `localhost`) and `navigator.mediaDevices.getUserMedia` — both are already assumed for the rest of the app

### Routing & layout

- `/scanner`, at `app/scanner/page.tsx` — a top-level route, **outside** the `(app)` guarded route group (see "Public, not behind login" above), sibling to the root `app/page.tsx` login page rather than nested under `(app)`
- The page provides its own minimal header (`BrandLogo` linking to `/`, `ColorModeButton`) instead of the `(app)` layout's `SideNav`/`TopNav`
- Add a `{ label: "Scanner", href: "/scanner" }` entry to `NAV_ITEMS` in `src/lib/nav.ts` (still useful as a shortcut while logged in, even though the destination page doesn't carry the app chrome)
- Do **not** add `/scanner` to the `proxy.ts` matcher — it must stay reachable without the `access_token`/`refresh_token` cookies
- Heading ("Scanner"). No create/list/pagination on this page — it's a single lookup tool, not a CRUD page
- Layout: camera view and the result card side by side (`Flex` with `wrap="wrap"`, each child `maxW="md"`), not stacked — the result card only renders once there's something to show, so the layout is just the camera alone until the first scan

### Camera scanning

- `BarcodeCameraScanner.tsx` wraps `BarcodeScanner` from `react-barcode-scanner`, loaded via `next/dynamic(..., { ssr: false })` since it touches `navigator`/camera APIs that don't exist server-side; the `react-barcode-scanner/polyfill` side-effect import is likewise only pulled in on the client (inside this component, not the app's root layout), since the polyfill is only needed for browsers lacking a native `BarcodeDetector` (e.g. Firefox, Safari) and only this page uses it
- `options.formats` restricted to the 1D formats medicine barcodes actually use: `['code_128', 'ean_13', 'ean_8', 'upc_a', 'upc_e']` — no `qr_code`, since generated medicine barcodes are 1D (see Rendering below)
- `trackConstraints={{ facingMode: "environment" }}` to prefer the rear camera on phones/tablets; falls back to whatever camera is available on desktop webcams
- On the first `onCapture` with a non-empty `DetectedBarcode[]`, take `barcodes[0].rawValue`, set the component to `paused` (stop feeding new detections) and trigger the lookup (see below) — the camera keeps re-firing `onCapture` for the same code every `delay` tick while it's in frame, so `paused` (rather than a second debounce) is what stops duplicate lookups for one scan
- Cooldown, not a manual reset: once a lookup settles (success or error), `ScannerPageClient` starts a timer and only sets `paused={false}` (resuming detection) once it fires — there is no "Scan Again" button, scanning re-arms itself. The delay is `NEXT_PUBLIC_SCANNER_COOLDOWN_SECONDS` (env var, `apps/app/.env`/`.env.example`; must be `NEXT_PUBLIC_` since the timer runs client-side), defaulting to 5 seconds if unset/invalid
- While paused (mid-lookup or cooling down), the camera view shows "Scanning paused — resumes automatically." instead of the usual hint text, so it doesn't look broken
- Camera errors, via the `BarcodeScanner` component's `onCameraError` prop:
  - `NotAllowedError` (permission denied): inline message asking the user to allow camera access and reload
  - Any other camera error (no camera, `BarcodeDetector` unsupported and polyfill still failing, etc.): same fallback message, generic wording
- There is no manual/typed entry fallback — the camera is the only input. A camera error just leaves the page without a way to look anything up (acceptable for this pass; revisit if that turns out to matter in practice)

### Lookup & result

- `ScanResultCard.tsx` renders one of three states after a lookup:
  - **Loading**: spinner while the request is in flight
  - **Found**: the medicine's details via the `DataList` snippet — Name, Barcode, Batch Number (em dash when `null`), Expiration Date, Quantity, Status (derived client-side via the existing `getMedicineStatus` from `src/lib/medicine-status.ts`, rendered with the `Status` snippet, same `STATUS_LABEL`/`STATUS_VALUE` mapping as `MedicinesTable.tsx`) — plus the barcode redrawn with `<Barcode value={medicine.barcode} format="CODE128" displayValue={false} />` from `react-barcode`. `format="CODE128"` (not `EAN13`/`UPC`) because medicine barcodes are arbitrary text up to 128 chars, not necessarily a checksummed numeric format — `CODE128` accepts any ASCII value JsBarcode can encode without a checksum mismatch throwing
  - **Not found**: `404` (`medicine not found`) renders the `Alert` snippet showing the scanned value and the server's error message, not a dead end — the camera automatically re-arms after the cooldown and the next scan replaces this state
- Other errors (`400` invalid barcode, `500`) also show via the `Alert` snippet with the server's message (or a generic one for `500`), same as the other pages' inline error handling
- Only one lookup result is shown at a time; a new scan while a previous result is displayed replaces it in place — the previous result/error just stays on screen until then, it isn't cleared when the cooldown ends

### Data fetching

- Add `getMedicineByBarcode(barcode: string)` to `src/actions/medicines.ts` → `GET /medicines/barcode/{barcode}`, `barcode` URL-encoded into the path (per the swagger description: "URL-encode reserved characters"); returns `ActionResult<Medicine>` following the same pattern as `createMedicine`/`updateMedicine` (`readErrorMessage`/`MESSAGE_STATUSES` already cover `400`/`401`/`404`, no changes needed there)
- No new type needed — reuses `Medicine` from `src/types/medicine.ts`
- Still forwards the `access_token` cookie as `Authorization: Bearer <token>` when present (same helper as the rest of `medicines.ts`), but the endpoint no longer requires it — a logged-out visitor has no cookie to forward, and the request succeeds anyway

### Components

Under `src/components/scanner/`:

- `ScannerPageClient.tsx` — owns the scanned barcode, lookup loading/result/error state, the cooldown timer, and whether the camera is paused; wires `BarcodeCameraScanner`'s captures to the lookup function and renders `ScanResultCard` side by side with it
- `BarcodeCameraScanner.tsx` — the dynamically-imported camera view, capture/pause logic, the paused/hint text, and camera error messaging described above
- `ScanResultCard.tsx` — loading/found/not-found/error rendering described above

## References

- @context/project-overview.md
- @context/features/13-medicines-page.spec.md
- @apps/api/internal/handler/swagger/openapi.json
- @libs/ts/snippets/src/lib/data-list.tsx
- @libs/ts/snippets/src/lib/status.tsx
- @libs/ts/snippets/src/lib/alert.tsx
- @apps/app/src/lib/medicine-status.ts
- @apps/app/src/components/medicines/MedicinesTable.tsx
- @apps/app/src/actions/medicines.ts
- @apps/app/src/types/medicine.ts
- @apps/app/src/types/action.ts
- @apps/app/src/lib/nav.ts
- @apps/app/src/proxy.ts
- https://www.npmjs.com/package/react-barcode-scanner
- https://www.npmjs.com/package/react-barcode
