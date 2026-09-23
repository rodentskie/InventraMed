### WebSocket Server

This will be on `apps/ws`.
A WebSocket server with one job: any message a connected client sends gets forwarded to every **other** connected client. It doesn't parse the payload, keep state, or apply business logic. It is the real-time bridge between the web app / API and the ESP32 LED indicators (see "Real-time sync" in @context/project-overview.md). What goes over the wire (message shape, who sends what) is decided by whichever feature adds the publishers and subscribers. This server just relays bytes.

Based on the hub/client pattern from `rodentskie/svm` `app/ws/main.go`, which in turn follows gorilla's chat example. The review changes listed below replace the parts of that code that break this project's standards or have bugs.

## Review of the reference implementation

Keep as-is (these are correct):

- Hub goroutine owns the `clients` map. Register, unregister and broadcast all go through channels, so the map needs no mutex
- One read pump and one write pump per client, so each connection has at most one reader and one writer (gorilla's concurrency rule)
- Keepalive: ping every `pingPeriod` (9/10 of `pongWait`). The pong handler extends the read deadline, and `writeWait` bounds every write
- `SetReadLimit` caps the size of inbound messages
- Buffered `send` channel per client (256). If a client's buffer is full, the hub drops it and closes its `send` instead of blocking the broadcast to everyone else
- The sender is excluded from its own broadcast (echoing back to the sender is not needed for LED sync)

Change (bugs or standards violations):

| # | Reference | Problem | Change |
|---|-----------|---------|--------|
| 1 | `var upgrader`, `var wsLogger` package globals | Breaks "no global state" | Inject the logger and upgrader through constructors |
| 2 | `CheckOrigin` always returns `true` | Any website can open a socket from a visitor's browser (cross-site WebSocket hijacking) | Check against the `WS_ALLOWED_ORIGINS` allowlist (see Config) |
| 3 | `Hub.Stop()` exists but nothing calls it; `ListenAndServe` has no graceful shutdown | SIGTERM drops sockets without a close frame | Use `signal.NotifyContext` → `server.Shutdown(ctx)` → `hub.Stop()` |
| 4 | After `Stop()`, `readPump`'s `hub.unregister <- c`, `hub.broadcast <- …` and `serveWS`'s `hub.register <- c` send on unbuffered channels that no one reads any more | Goroutine leak / deadlock during shutdown | Every send to the hub `select`s on `<-h.stop` as well (private `join`/`leave`/`publish` methods that return early once stopped) |
| 5 | Every outbound frame is written as `websocket.TextMessage` | A binary frame is relayed as text, which corrupts it | Carry the message type in the broadcast and write with the same type it was received as |
| 6 | `http.Server` has no `ReadHeaderTimeout` | Slowloris exposure; `gosec` G112 | Set `ReadHeaderTimeout` |
| 7 | Logger is `library/go/logger` | Not this project's logger | Use `github.com/rodentskiedev/go-libraries/lib/logger` (`logger.New()`, `defer log.Sync()`) |
| 8 | Hardcoded `":8081"` | Standards require configuration from ENV | Read `PORT` and friends through `env.GetEnv` in `config.LoadConfig()` |
| 9 | Everything in one `main.go` | Standards want `cmd/` entry points and `internal/` packages | See File organization |

## Requirements

### Behaviour

- One endpoint, `GET /ws`, which upgrades the connection to a WebSocket. No `/api/v1` prefix, because this isn't a REST resource and clients connect to a fixed socket URL
- Any text or binary message received from a client goes to every other connected client, keeping the same frame type. The payload is opaque and is never parsed or validated
- No authentication in this pass. The ESP32 connects over the LAN and there is no auth handshake yet (see Out of scope)
- Messages that arrive while no other client is connected are dropped. There is no persistence, replay, or "last known state" for late joiners
- A client that is too slow (its `send` buffer is full) is disconnected. This keeps one stuck ESP32 from stalling everyone else
- Ping/pong keepalive detects dead peers, for example an ESP32 that lost power without sending a close frame, and removes them within `pongWait`

### Constants (protocol tuning, not deployment config, so they stay in code)

- `writeWait = 10s`, `pongWait = 60s`, `pingPeriod = pongWait * 9 / 10`, `maxMessageSize = 8 KiB`, `sendBufferSize = 256`

### Config (`apps/ws/config/config.go`)

`LoadConfig() (Config, error)`, following the same pattern as `apps/api/config`:

| ENV | Default | Notes |
|-----|---------|-------|
| `PORT` | `8081` | Different from the API's `8080` so both can run locally side by side |
| `WS_ALLOWED_ORIGINS` | `""` | Comma-separated list, e.g. `http://localhost:3000,https://inventramed.example`. Entries are trimmed and empty ones ignored. `*` allows any origin |
| `WS_SHUTDOWN_TIMEOUT` | `10s` | Parsed with `time.ParseDuration`. An invalid value makes `LoadConfig` return an error |

- Add `apps/ws/.env.example` listing the variables above

### Origin check

- **No `Origin` header** → allowed. Non-browser clients like the ESP32 don't send one, and browsers always do, so this doesn't open a cross-site hole
- `Origin` header present → allowed only if it exactly matches (case-insensitive) an entry in `WS_ALLOWED_ORIGINS`, or if the list contains `*`
- A rejected upgrade gets `403` (gorilla's `Upgrade` writes the error response) and is logged at `Warn` with the origin

### Shutdown order

1. `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`
2. `server.Shutdown(ctx with WS_SHUTDOWN_TIMEOUT)`, which stops accepting new connections. Hijacked WebSocket connections are **not** closed by `Shutdown`, so:
3. `hub.Stop()`: the hub closes every client's `send`, so each write pump sends a close frame (`1001 going away`) and closes its conn. `Stop` blocks until every write pump has exited (each write is bounded by `writeWait`), so the close frames go out before the process exits. The read pumps then error out and their leave/publish sends return immediately because of the `stop` select

### File organization

```
apps/ws/
  cmd/ws/main.go                 # wires config, logger, hub, handler, server, shutdown
  config/config.go               # Config + LoadConfig
  config/config_test.go
  internal/hub/hub.go            # Hub: Run, Stop, ServeConn (+ private join, leave, publish)
  internal/hub/client.go         # client: readPump, writePump
  internal/hub/hub_test.go
  internal/handler/ws/handler.go # upgrader + origin check, Serve(w, r)
  internal/handler/ws/handler_test.go
  internal/wstest/wstest.go      # test-only dial/read helpers shared by hub and handler tests
  .env.example
```

- The handler depends on a small `ConnectionServer` interface (`ServeConn(*websocket.Conn)`, which `*hub.Hub` implements) rather than on `*hub.Hub`, per the layered-architecture rule. The `client` type and the hub's join/leave/publish methods stay unexported. There is no service or repository layer because there is no business logic or storage
- The placeholder `apps/ws/main.go` / `main_test.go` (the `Hello` scaffold) is replaced by `cmd/ws/main.go`
- `project.json`: `build` gets `"main": "./cmd/ws"`, add a `dev` target (`gow run ./cmd/ws`, same as `api`), and `test` gets `"cover": true`
- Add `github.com/gorilla/websocket`, `github.com/rodentskiedev/go-libraries/lib/env` and `.../lib/logger` through `tidy` (never edit `go.mod` by hand)

### Tests (≥90% coverage target)

- `config`: defaults, origin list parsing (trimming, empty entries, `*`), invalid `WS_SHUTDOWN_TIMEOUT`
- `hub`: a broadcast reaches the other clients and not the sender; a client with a full buffer is dropped and its `send` closed; `Stop` closes every client's `send`; join/leave/publish after `Stop` return without blocking; `Stop` sends a `1001` close frame to connected sockets; an oversized message closes the sender with `1009` and is not relayed
- `handler/ws`, using `httptest.NewServer` + `websocket.Dialer`: no `Origin` → upgrades; allowed origin → upgrades; disallowed origin → `403`; `*` → any origin upgrades; end-to-end, client A sends text and binary, client B receives both with the same frame type and A receives nothing
- Run the tests with `-race`, since the hub is all goroutines and channels

## Out of scope

- Authentication/authorization of socket clients (for example a shared token for the ESP32, or JWT for browsers)
- Rooms/topics, message schema and validation, persistence/replay
- Having `apps/api` or `apps/app` publish to the socket, and the ESP32 firmware. Those come in their own features

## References

- @context/project-overview.md
- @context/go-standards.md
- @apps/api/config/config.go
- @apps/api/cmd/api/main.go
- @apps/api/project.json
- https://raw.githubusercontent.com/rodentskie/svm/refs/heads/main/app/ws/main.go
- https://github.com/gorilla/websocket/tree/main/examples/chat
