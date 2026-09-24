### Live View — Phase 2 (Location + Scanner → WebSocket → Live View)

This touches `apps/migration`, `apps/api` and `apps/app`. `apps/ws` gets **no code changes**.

Phase 1 (@context/features/19-live-phase-1.spec.md) built the 3D tray with 12 compartments and a green/yellow/red LED cluster above each one. Phase 2 connects the system to the WebSocket server (`apps/ws`, see @context/features/18-ws-server.spec.md) so a scan can light the LED for the right compartment:

1. Each medicine gets a **location**: which of the 12 tray compartments it sits in
2. The API stores and returns the location on every medicine endpoint
3. The medicines page lets users set and see the location
4. When the **scanner page** finds a medicine, it builds a small JSON message with a message type, the medicine's location and its status, and sends it to `apps/ws`
5. `apps/ws` relays the message to every other connected client (the ESP32 and the live view). It already does this, and does nothing else
6. The **live view** (`/live`) listens on `apps/ws` and lights the LED for the scanned compartment in real time

## Flow

```
Scanner page (browser)
  │  1. camera reads barcode
  │  2. GET /medicines/barcode/{barcode}  ──►  apps/api  (returns medicine + location)
  │  3. success only: build { type: "scan", location, status }
  │  4. send JSON over WebSocket
  ▼
apps/ws  (WS_SERVER, e.g. ws://localhost:9000/ws)
  │  relays to every client except the sender, never parses the payload
  ▼
ESP32   →  light green / yellow / red on compartment `location`
/live   →  light the same LED on the 3D tray
```

- **The scanner page is the only sender.** Nothing else in `apps/app` or `apps/api` writes to the socket in this phase. `/live` only listens
- A message is sent **only after a successful lookup**, i.e. the API returned a medicine. Not-found (404), validation errors (400), network errors and 500s send nothing

## 1. Migration: `medicines.location`

New file `apps/migration/transactions/00025_add_medicines_location.sql`:

```sql
-- +goose Up
ALTER TABLE medicines
    ADD COLUMN location smallint
        CONSTRAINT chk_medicines_location CHECK (location BETWEEN 1 AND 12);

CREATE UNIQUE INDEX uq_medicines_location
    ON medicines (location)
    WHERE deleted_at IS NULL AND location IS NOT NULL;

-- +goose Down
DROP INDEX uq_medicines_location;
ALTER TABLE medicines DROP COLUMN location;
```

- `location` is the compartment number, **1–12**, using the same numbering as the live view's `CompartmentState.id`: row-major from the back-left pocket as seen from the front. The tray has exactly 12 compartments
- **Nullable.** `NULL` means "not placed in the tray". Existing rows become `NULL`, so the migration needs no backfill
- **One compartment, one medicine.** `uq_medicines_location` allows each location on at most one active medicine:
  - Partial on `deleted_at IS NULL`, so soft-deleting a medicine frees its compartment (the same approach as `uq_medicines_name_batch`)
  - Any number of medicines can be unplaced (`NULL`)
- The range and uniqueness are enforced **at both levels**. The API checks first and returns a friendly `400`/`409`. The `CHECK` constraint and the unique index are the last line of defence, e.g. against two concurrent requests for the same compartment
- `medicines` has an `updated_at` trigger (00023) but no audit trigger, so nothing else needs to change

## 2. API: location on create, update and list

`apps/api` gets these changes. The layering stays the same (handler → service → repository).

### Domain (`internal/domain/medicine.go`)

- Add `Location *int` to `domain.Medicine`. `nil` means not placed

### Repository (`internal/repository/medicine/repository.go`)

- Add ``Location *int `gorm:"type:smallint"` `` to `record`, and map it in `toRecord` / `toDomain`
- New method `ExistsByLocation(ctx, location int, excludeID string) (bool, error)`: whether an **active** medicine has the location. Not `Unscoped()`, unlike `ExistsByBarcode`, because soft-deleted medicines don't hold a compartment. A non-empty `excludeID` skips that medicine
- `translateError`: map the `uq_medicines_location` constraint (new const `locationConstraint`) to `apperror.ErrLocationTaken`
- `Update`: add `"location"` to the `Select(...)` list and to the `record{...}` passed to `Updates`, so clearing it stores `NULL` (the same reason `batch_number` is in the list)
- Update the `Update` doc comment: it writes the name, barcode, batch number, expiration date **and location**, but never the quantity
- `List` and `FindByBarcode` need no query changes. They select the whole record, so the new field comes back automatically

### Service (`internal/service/medicine/service.go`)

- Add `Location *int` to `CreateInput` and `UpdateInput`, and pass it through to the repository in `Create` and `Update`
- `checkUnique` gets the location and checks it **last**, after name/batch and barcode: when it isn't `nil` and `ExistsByLocation` is true, return `apperror.ErrLocationTaken`. On update the medicine's own ID is excluded, so saving it with its current location is fine
- Moving a medicine to a taken compartment is a `409`. Users free the compartment first (edit the other medicine to "Not placed" or another location). No automatic swap

### Errors (`pkg/apperror/apperror.go`)

- Add `ErrLocationTaken = fmt.Errorf("medicine location already taken: %w", ErrConflict)` next to the other medicine conflicts, so the service's `fail` passes it through unchanged

### Handler (`internal/handler/medicine/`)

- New constants: `minLocation = 1`, `maxLocation = 12`
- `createRequest` and `updateRequest` get ``Location *int `json:"location"` ``
  - Omitted or `null` → not placed (`nil`)
  - An integer outside `1..12` → `400` `"location must be between 1 and 12"`
  - A non-integer (e.g. `"7"`, `7.5`) already fails JSON decoding → `400` `"invalid request body"`, same as today
- Validate it in one helper, `validateLocation(*int) string`, called from both `decodeCreate` and `decodeUpdate`. `validateDetails` keeps its current signature
- `writeError`: `ErrLocationTaken` → `409` `"another medicine is already in this location"`
- `PUT /medicines/{id}` is a full replace, like `batch_number`: omitting `location` clears it. The app always sends the field (see section 3)
- `medicineResponse` gets ``Location *int `json:"location"` ``, always present, `null` when not placed. This covers:
  - `POST /medicines` (the `data` of the create response)
  - `GET /medicines` (each item of the list)
  - `GET /medicines/barcode/{barcode}`, the scanner's lookup (see section 4)
- There is no `GET /medicines/{id}` endpoint today. "Get one" is the barcode endpoint; no new endpoint is added

### Swagger (`internal/handler/swagger/openapi.json`)

- `Medicine` schema: add `location` to `required` and to `properties`:
  ```json
  "location": {
    "type": "integer",
    "minimum": 1,
    "maximum": 12,
    "nullable": true,
    "description": "Tray compartment (1–12, row-major from the back-left pocket as seen from the front). null when the medicine is not placed in the tray.",
    "example": 7
  }
  ```
- `CreateMedicineRequest` and `UpdateMedicineRequest`: add the same `location` property, **not** in `required`. Update request description: "Optional. An omitted or null value clears the location."
- Location description on both requests also says: "Must not be used by another active medicine."
- Add the new `400` message to the documented validation errors wherever the create/update `400` responses list them, and the new `409` message wherever the create/update `409` responses list them

### Tests

- Handler: location valid (1, 12), out of range (0, 13, -1) → `400` with the message, omitted/`null` → `nil` passed to the service, `ErrLocationTaken` → `409`, `location` present in create/list/barcode responses (`null` and a number)
- Service: `Location` is passed through on create and update; taken location → `ErrLocationTaken` on create and update; `nil` location skips the check; on update the medicine's own ID is excluded; check order is name/batch → barcode → location
- Repository (`repository_sql_test.go`, sqlmock): the update statement includes `location`, a `nil` location is written as `NULL`, `ExistsByLocation` filters out soft-deleted rows and honours `excludeID`, and a unique violation on `uq_medicines_location` maps to `ErrLocationTaken`
- Coverage stays at the current level (≥ 90%)

## 3. App: medicines page

`apps/app/src/types/medicine.ts`:

- `Medicine`: add `location: number | null`
- `CreateMedicineInput` and `UpdateMedicineInput`: add `location: number | null`

`MedicineFormDrawer.tsx`:

- New optional **Location** field using the `native-select` snippet, with options "Not placed" (empty value → `null`) and `#1` … `#12`
- Shown in **both** create and edit mode (unlike Quantity)
- Pre-filled from `medicine.location` when editing
- Always sent in the create and update payloads (`null` when "Not placed"), so an update never clears it by accident
- A `409` for a taken location shows in the drawer's existing error `Alert`, like the other conflicts. No extra handling needed: `actions/medicines.ts` already surfaces the API's `409` message
- Keep the option list in a constant (`LOCATIONS`, 1–12) next to the other live constants in `src/lib/live.ts`, not inline. It's the same 12 compartments the live view renders

`MedicinesTable.tsx`:

- New **Location** column between "Batch Number" and "Expiration Date", showing `#7`, or `—` (the existing `EMPTY_VALUE`) when `null`. `#<n>` matches the live view's hover label

`actions/medicines.ts` needs no logic changes. The new field travels through the typed inputs and responses.

## 4. API: barcode lookup includes location

Covered by section 2: `GET /medicines/barcode/{barcode}` returns `toResponse(found)`, so adding `Location` to `medicineResponse` includes it. Called out separately because the scanner depends on it. Add a handler test asserting `location` is in this response.

`ScanResultCard.tsx`: add a **Location** row (`#7`, or `—` when not placed) after "Batch Number".

## 5. Scanner: the scan message

A scan message tells listeners which compartment's LED to light and which color:

```json
{ "type": "scan", "location": 7, "status": "near" }
```

| Field | Type | Meaning |
|-------|------|---------|
| `type` | `"scan"` | The kind of message. Always `"scan"` in this phase |
| `location` | integer, 1–12 | The compartment (`medicine.location`) |
| `status` | `"good"` \| `"near"` \| `"expire"` | Which LED to light |

The status comes from the existing `getMedicineStatus(expiration_date)` (`src/lib/medicine-status.ts`, 30-day warning threshold), mapped to the wire value:

| `MedicineStatus` | `status` | LED |
|------------------|----------|-----|
| `green` | `good` | Green, far from expiration |
| `yellow` | `near` | Yellow, nearing expiration |
| `red` | `expire` | Red, expired |

Types in `src/types/live.ts`:

```ts
export type ScanStatus = "good" | "near" | "expire"

export interface ScanMessage {
  type: "scan"
  location: number // 1–12, same numbering as CompartmentState.id
  status: ScanStatus
}
```

Helpers in `src/lib/live.ts`:

- `SCAN_STATUS: Record<MedicineStatus, ScanStatus>`: the mapping table above
- `toScanMessage(medicine: Medicine, now?: Date): ScanMessage | null`: returns `null` when `medicine.location` is `null`, since there is no LED to light
- Pure functions, so they're easy to unit test once an app test runner exists

Rules:

- **Receivers check `type` first** and ignore any message whose `type` they don't recognise. Every client on `apps/ws` gets every message, so this lets new kinds of message (e.g. a snapshot for a rebooted ESP32, or a self-test) be added later without reflashing the firmware. The literal `"scan"` is a named constant (`SCAN_MESSAGE_TYPE`) in `src/lib/live.ts`
- **Medicine without a location → nothing is sent.** The scan result card still shows the medicine as usual, with Location `—`
- The status is computed in the browser at scan time. The API doesn't return a status, and this phase doesn't add one

## 6. Scanner: send to `apps/ws`

### Env

`apps/app/.env.example` (and the local `.env`):

```
# WebSocket server the scanner page sends scan messages to (apps/ws).
# Read on the server at request time and passed to the page, so it
# doesn't need a NEXT_PUBLIC_ prefix and can change without a rebuild.
WS_SERVER=ws://localhost:9000/ws
```

- The browser opens the socket, but the variable is **not** `NEXT_PUBLIC_`. `app/scanner/page.tsx` (a server component) calls `await connection()` from `next/server` and then reads `process.env.WS_SERVER`, so it's evaluated per request and not inlined at build time (the Next 16 pattern for runtime env). It passes the value to `<ScannerPageClient wsUrl={...} />`
- Empty or unset `WS_SERVER` → the scanner works as today and never connects. No error is shown
- `apps/ws/.env.example` already allows `http://localhost:3000` in `WS_ALLOWED_ORIGINS`. Any other origin the app is served from (a LAN IP, production) has to be added there. This is config only, not a code change

### Library

Add `react-use-websocket` (v4, the React 18+ line) with yarn, as phase 1 planned. It handles connect, reconnect and `readyState`. We use:

- `useWebSocket(url, options, shouldConnect)` with `shouldConnect = Boolean(wsUrl)`
- `shouldReconnect: () => true`, a fixed `reconnectInterval` (3 s) and no attempt limit, because the scanner page may stay open all day while `apps/ws` restarts
- `sendJsonMessage(message, false)`: `keep = false`, so a scan made while disconnected is **dropped, not queued**. A queued scan would light an LED minutes later for a medicine nobody is holding
- `readyState` (compared with the library's `ReadyState.OPEN`) to tell whether the send can go out. `sendJsonMessage` doesn't report failures itself

### Failed send → toast

- `publish` checks `readyState` first. When the socket isn't `OPEN` (still connecting, reconnecting, or `apps/ws` is down), it skips the send and shows a toast:
  - `toaster.create({ type: "warning", title: "Couldn't update the tray LEDs", description: "The live server is unreachable. The scan result is still shown." })`
  - `warning`, not `error`: the scan itself succeeded, and only the LED update failed
- The `Toaster` is mounted in the root `app/layout.tsx`, so it works on the public scanner page too
- No toast when:
  - `WS_SERVER` is unset: the connection is turned off on purpose
  - The medicine has no location: nothing needed sending
- A send on an `OPEN` socket counts as sent. The browser gives no delivery confirmation, and `apps/ws` sends no acknowledgement

### Components

- `src/components/scanner/useScanPublisher.ts`: a hook that takes `wsUrl` and returns `publish(medicine: Medicine)`. It calls `toScanMessage`, and if the result isn't `null` it sends it, or shows the toast when the socket isn't open (see "Failed send → toast"). This keeps socket details out of `ScannerPageClient`, the same way `useMedicineLookup` wraps its logic for purchase orders
- `ScannerPageClient.tsx`: takes a `wsUrl: string` prop. In `lookup`, after `setMedicine(result.data)` on the success branch **only**, call `publish(result.data)`
- `app/scanner/page.tsx`: becomes `async`, calls `connection()`, reads `WS_SERVER` and passes it down. It stays public (outside `(app)`), as today

The socket connects when the scanner page mounts, not on the first scan, so the first message isn't lost while connecting. It closes when the page unmounts (the hook handles this).

### Testing

- `apps/api`: tidy, lint, build, test via Nx
- `apps/migration`: run `up` and `down` against a local database
- `apps/app`: lint and build via Nx (there's still no unit-test runner), plus a manual check:
  - Run `apps/ws` and connect a second client (e.g. `websocat ws://localhost:9000/ws`)
  - Scan a medicine with location `7` that expires in 10 days → the second client receives `{"type":"scan","location":7,"status":"near"}`
  - Fresh → `good`, expired → `expire`
  - Unknown barcode → nothing received
  - Medicine without a location → nothing received, card shows Location `—`
  - Stop `apps/ws`, then scan → warning toast. Start it again → the scan made while it was down is not delivered, and the next scan is
  - Unset `WS_SERVER` → scanner still works, no connection attempts in the network tab, no toast
  - Medicines page: create and edit with and without a location. The table and scan card show `#n` / `—`
  - Give a second medicine a location that's already used → `409` message in the drawer. Soft-delete the first medicine → the location can be reused

## 7. Live view: listen to `apps/ws`

`/live` connects to the same `WS_SERVER` and turns each scan message into LED state on the 3D tray, replacing phase 1's "not connected" preview. It never sends anything.

### No history

`apps/ws` doesn't replay old messages, so `/live` starts with **all LEDs off** and lights compartments as scans arrive. To see a scan, `/live` must already be open when it happens. This is intended: open `/live` (e.g. on a wall display) before scanning starts.

### Page

- `app/(app)/live/page.tsx`: becomes `async`, calls `await connection()`, reads `process.env.WS_SERVER ?? ""` and passes it as `<LivePageClient wsUrl={...} />`, the same as `/scanner`. It stays behind login, inside `(app)`

### Parsing messages

In `src/lib/live.ts`:

- `MEDICINE_STATUS: Record<ScanStatus, MedicineStatus>`: the inverse of `SCAN_STATUS` (`good` → `green`, `near` → `yellow`, `expire` → `red`)
- `parseScanMessage(data: string): ScanMessage | null`: `JSON.parse` in a `try`, then returns the message only when **all** of these hold, otherwise `null`:
  - it's an object with `type === SCAN_MESSAGE_TYPE`
  - `location` is an integer in `LOCATIONS` (1–12)
  - `status` is a key of `MEDICINE_STATUS`
- Anything else is ignored silently: non-JSON, binary frames, unknown `type`, bad fields. Every client gets every message, so unknown messages are expected, not errors

### Hook

`src/components/live/useScanSubscriber.ts`: `useScanSubscriber(wsUrl: string, onScan: (message: ScanMessage) => void): LiveConnection`

- `useWebSocket(enabled ? wsUrl : null, options, enabled)` with the same reconnect settings as the scanner (every 3 s, no attempt limit)
- `onMessage`: `parseScanMessage(event.data)`, and call `onScan` when it isn't `null`
- `filter: () => false`: the library calls `onMessage` before `filter`, so every message is still handled, but `lastMessage` never updates and the page doesn't re-render on each message. LED updates re-render through `onScan` only
- Returns the connection state for the badge:

```ts
// src/types/live.ts
export type LiveConnection = "off" | "connecting" | "live" | "disconnected"
```

| `readyState` | `LiveConnection` |
|--------------|------------------|
| `WS_SERVER` unset | `off` |
| `CONNECTING`, `UNINSTANTIATED` | `connecting` |
| `OPEN` | `live` |
| `CLOSING`, `CLOSED` | `disconnected` (the hook keeps retrying) |

### Applying a scan

In `LivePageClient`:

- `applyScan(message)` sets compartment `message.location` to `statusToLeds(MEDICINE_STATUS[message.status])`. That lights exactly one LED and turns the other two off, so a rescan after a status change replaces the old color
- Other compartments are left as they are. A compartment stays lit until another scan for it arrives or the page reloads
- The comment "the WebSocket will in a later phase" becomes accurate: the socket writes into the same `compartments` state as the simulation panel

### Connection badge

The phase 1 badge ("Design preview · not connected") shows the real state, using the `Status` snippet:

| `LiveConnection` | Text | `Status` value |
|------------------|------|----------------|
| `off` | "Not connected" | `colorPalette="gray"` |
| `connecting` | "Connecting…" | `warning` |
| `live` | "Live" | `success` |
| `disconnected` | "Disconnected · retrying" | `error` |

Labels and values live in a constant map in `LivePageClient`, like `LEGEND_LABEL`.

### Simulation panel

- `LedSimulationPanel` stays as a dev tool, hidden by `NEXT_PUBLIC_HIDE_LIVE_PRESETS` as before. Both it and the socket write into `compartments`
- Its toggles don't follow socket updates: after a scan, the leva checkboxes can show a stale value. Acceptable for a hidden dev tool

### Testing

- Lint and build via Nx
- Manual:
  - Open `/live`: the badge goes "Connecting…" → "Live", all LEDs are off
  - Scan a medicine in `#7` on `/scanner` (another tab or device): compartment 7 lights the matching color on `/live`, with no reload
  - Rescan after changing its expiration date so its status changes: the old LED goes off and the new one lights
  - Send junk from a second client (`websocat`: `hello`, `{"type":"other"}`, `{"type":"scan","location":13,"status":"good"}`): nothing changes, no errors in the console
  - Stop `apps/ws`: badge "Disconnected · retrying". Start it: badge back to "Live" within a few seconds, and lit LEDs stay lit
  - Unset `WS_SERVER`: badge "Not connected", no connection attempts

## Out of scope

- Any code change in `apps/ws`. It stays a blind relay
- Showing scans that happened before `/live` was opened (no replay, see "No history")
- ESP32 firmware
- A full-state snapshot for late joiners (the ESP32 only learns about compartments as they're scanned)
- A persistent connection-status indicator on the scanner page (only the failed-send toast)
- Authentication on the socket
- Filtering or sorting the medicines list by location
- Sending a message on create, update or delete of a medicine

## Decisions

- `/live` listens to `apps/ws` and lights LEDs in real time. It shows only scans made while it's open
- Every scan message has `"type": "scan"`, and receivers ignore types they don't recognise

- One compartment holds at most one active medicine, enforced by the database (`uq_medicines_location`) and the API (`409`)
- A scan whose message can't be sent shows a warning toast
- The tray has exactly 12 compartments, enforced by the database (`CHECK`) and the API (`400`)

## References

- @context/project-overview.md
- @context/features/18-ws-server.spec.md
- @context/features/19-live-phase-1.spec.md
- @apps/migration/transactions/00007_create_medicines_table.sql
- @apps/api/internal/domain/medicine.go
- @apps/api/internal/repository/medicine/repository.go
- @apps/api/internal/service/medicine/service.go
- @apps/api/internal/handler/medicine/handler.go
- @apps/api/internal/handler/medicine/update.go
- @apps/api/internal/handler/medicine/list.go
- @apps/api/internal/handler/swagger/openapi.json
- @apps/app/src/types/medicine.ts
- @apps/app/src/types/live.ts
- @apps/app/src/lib/live.ts
- @apps/app/src/lib/medicine-status.ts
- @apps/app/src/components/medicines/MedicineFormDrawer.tsx
- @apps/app/src/components/medicines/MedicinesTable.tsx
- @apps/app/src/components/scanner/ScannerPageClient.tsx
- @apps/app/src/components/scanner/ScanResultCard.tsx
- @apps/app/src/app/scanner/page.tsx
- @apps/app/src/app/(app)/live/page.tsx
- @apps/app/src/components/live/LivePageClient.tsx
- @apps/app/src/components/live/LedSimulationPanel.tsx
- @apps/ws/.env.example
- https://github.com/robtaussig/react-use-websocket
- https://nextjs.org/docs/app/guides/environment-variables#runtime-environment-variables
